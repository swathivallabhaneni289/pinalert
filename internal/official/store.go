package official

// Storage of one alert version. saveVersion runs inside the caller's
// transaction so that the alert row, its areas, the cancelled and superseded
// marks and the version hook (plan 05-11 queues phone alerts there) commit
// together or not at all: a crash can neither lose a queued alert nor repeat
// one.
//
// Text is stored exactly as CleanText left it, and only ever shown through
// html/template or textContent.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

// SavedVersion is a stored version of an alert: its row id, the version and
// where it was placed.
type SavedVersion struct {
	ID        int64
	Version   Version
	Placement Placement
}

// saveVersion stores one version of one series. It returns the stored version
// and, when the guid was stored before, the version it replaced (nil for a
// series seen the first time). q may be bound to tx or not; the work is done
// through tx either way.
//
//   - Alert and Update upsert the row by guid and replace its areas. An Update
//     also marks every stored alert of another guid whose cap_identifier is in
//     its references as superseded.
//   - Cancel marks every stored alert whose cap_identifier is in its references
//     as cancelled, and stores its own row with cancelled_at set, so it never
//     shows and is not fetched again. The rows are kept.
//   - An Alert or Update is then marked cancelled or superseded when a message
//     of another guid already stored names its identifier, so the order of
//     storing never decides whether a withdrawn alert shows.
//
// Nothing here removes an alert because it is missing from the feed.
func saveVersion(ctx context.Context, tx pgx.Tx, q *sqlcgen.Queries, v Version, p Placement, now time.Time) (SavedVersion, *SavedVersion, error) {
	qt := q.WithTx(tx)

	var prev *SavedVersion
	row, err := qt.OfficialAlertForUpdate(ctx, v.GUID)
	switch {
	case err == nil:
		pv, perr := savedFromRow(ctx, qt, row)
		if perr != nil {
			return SavedVersion{}, nil, perr
		}
		prev = &pv
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return SavedVersion{}, nil, fmt.Errorf("official: reading the stored series: %w", err)
	}

	cancel := v.MsgType == MsgCancel
	params := upsertParams(v, p, now)
	if cancel {
		if len(v.References) > 0 {
			if _, err := qt.CancelOfficialAlertsByIdentifier(ctx, sqlcgen.CancelOfficialAlertsByIdentifierParams{
				Now: now, Identifiers: v.References}); err != nil {
				return SavedVersion{}, nil, fmt.Errorf("official: marking cancelled alerts: %w", err)
			}
		}
		params.CancelledAt = pgtype.Timestamptz{Time: now, Valid: true}
	}

	stored, err := qt.UpsertOfficialAlert(ctx, params)
	if err != nil {
		return SavedVersion{}, nil, fmt.Errorf("official: storing the alert: %w", err)
	}
	if err := qt.DeleteOfficialAlertAreas(ctx, stored.ID); err != nil {
		return SavedVersion{}, nil, fmt.Errorf("official: clearing the areas: %w", err)
	}
	for _, code := range p.DistrictCodes {
		d := int32(code)
		if err := qt.InsertOfficialAlertArea(ctx, sqlcgen.InsertOfficialAlertAreaParams{AlertID: stored.ID, DistrictLgd: &d}); err != nil {
			return SavedVersion{}, nil, fmt.Errorf("official: storing a district: %w", err)
		}
	}
	for _, code := range p.StateCodes {
		s := int32(code)
		if err := qt.InsertOfficialAlertArea(ctx, sqlcgen.InsertOfficialAlertAreaParams{AlertID: stored.ID, StateLgd: &s}); err != nil {
			return SavedVersion{}, nil, fmt.Errorf("official: storing a state: %w", err)
		}
	}
	if v.MsgType == MsgUpdate && len(v.References) > 0 {
		if _, err := qt.SupersedeOfficialAlertsByIdentifier(ctx, sqlcgen.SupersedeOfficialAlertsByIdentifierParams{
			Now: now, Identifiers: v.References, Guid: v.GUID}); err != nil {
			return SavedVersion{}, nil, fmt.Errorf("official: marking replaced alerts: %w", err)
		}
	}
	// A Cancel or an Update stored before this alert (the refresher stores the
	// newest first), or before this guid was fetched again with the same
	// identifier, still withdraws it: the upsert above cleared the marks.
	if !cancel {
		if err := qt.MarkWithdrawnByStored(ctx, sqlcgen.MarkWithdrawnByStoredParams{ID: stored.ID, Now: now}); err != nil {
			return SavedVersion{}, nil, fmt.Errorf("official: applying stored withdrawals: %w", err)
		}
	}

	return SavedVersion{ID: stored.ID, Version: v, Placement: p}, prev, nil
}

func upsertParams(v Version, p Placement, now time.Time) sqlcgen.UpsertOfficialAlertParams {
	return sqlcgen.UpsertOfficialAlertParams{
		Guid:                v.GUID,
		CapIdentifier:       v.CapIdentifier,
		SourcePubdate:       timestamptz(&v.SourcePubDate),
		Sender:              v.Sender,
		Issuer:              v.IssuerLabel,
		MsgType:             string(v.MsgType),
		ReferencesText:      strings.Join(v.References, " "),
		Event:               v.Event,
		Severity:            string(v.Severity),
		Urgency:             v.Urgency,
		Certainty:           v.Certainty,
		SentAt:              v.SentAt,
		EffectiveAt:         timestamptz(v.EffectiveAt),
		OnsetAt:             timestamptz(v.OnsetAt),
		ExpiresAt:           v.ExpiresAt,
		ExpiresInferred:     v.ExpiresInferred,
		HeadlineEn:          v.HeadlineEN,
		HeadlineOriginal:    v.HeadlineOriginal,
		OriginalLang:        v.OriginalLang,
		DescriptionEn:       v.DescriptionEN,
		DescriptionOriginal: v.DescriptionOriginal,
		Instruction:         v.Instruction,
		AreaDesc:            v.AreaDesc,
		PlacementTier:       string(p.Tier),
		Now:                 now,
	}
}

// timestamptz is NULL for a nil or zero time.
func timestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil || t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func optionalTime(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time.UTC()
	return &t
}

// savedFromRow rebuilds the version and placement a stored row stands for.
func savedFromRow(ctx context.Context, q *sqlcgen.Queries, row sqlcgen.OfficialAlert) (SavedVersion, error) {
	areaRows, err := q.OfficialAlertAreas(ctx, row.ID)
	if err != nil {
		return SavedVersion{}, fmt.Errorf("official: reading the areas: %w", err)
	}
	p := Placement{Tier: Tier(row.PlacementTier)}
	for _, a := range areaRows {
		if a.DistrictLgd != nil {
			p.DistrictCodes = append(p.DistrictCodes, int(*a.DistrictLgd))
		}
		if a.StateLgd != nil {
			p.StateCodes = append(p.StateCodes, int(*a.StateLgd))
		}
	}
	v := Version{
		GUID:                row.Guid,
		CapIdentifier:       row.CapIdentifier,
		Sender:              row.Sender,
		IssuerLabel:         row.Issuer,
		SentAt:              row.SentAt.UTC(),
		Status:              "Actual",
		Scope:               "Public",
		MsgType:             MsgType(row.MsgType),
		References:          strings.Fields(row.ReferencesText),
		Event:               row.Event,
		Severity:            Severity(row.Severity),
		Urgency:             row.Urgency,
		Certainty:           row.Certainty,
		EffectiveAt:         optionalTime(row.EffectiveAt),
		OnsetAt:             optionalTime(row.OnsetAt),
		ExpiresAt:           row.ExpiresAt.UTC(),
		ExpiresInferred:     row.ExpiresInferred,
		HeadlineEN:          row.HeadlineEn,
		HeadlineOriginal:    row.HeadlineOriginal,
		OriginalLang:        row.OriginalLang,
		DescriptionEN:       row.DescriptionEn,
		DescriptionOriginal: row.DescriptionOriginal,
		Instruction:         row.Instruction,
		AreaDesc:            row.AreaDesc,
		DistrictCodes:       p.DistrictCodes,
	}
	if row.SourcePubdate.Valid {
		v.SourcePubDate = row.SourcePubdate.Time.UTC()
	}
	return SavedVersion{ID: row.ID, Version: v, Placement: p}, nil
}
