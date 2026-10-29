package service_test

// commentlikes_test.go proves that a like is one row per account per comment and
// that neither a like nor a comment flag can move any trust number (Phase 8 plan
// 09, D-02, D-16, T-08-43): a static scan of the files that build trust, and a
// byte equal comparison of BuildTrust before and after ten likes and a flag.

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// cmsSideRows counts the rows a like, an unlike or a flag must never write: the
// in app updates and the two phone outboxes.
func cmsSideRows(t *testing.T, r *cmsRig) int {
	t.Helper()
	var n int
	err := r.pool.QueryRow(context.Background(),
		`SELECT (SELECT count(*) FROM updates)
		      + (SELECT count(*) FROM push_outbox)
		      + (SELECT count(*) FROM official_push_outbox)`).Scan(&n)
	if err != nil {
		t.Fatalf("counting update and outbox rows: %v", err)
	}
	return n
}

func (r *cmsRig) likeRows(t *testing.T, commentID int64) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM comment_likes WHERE comment_id = $1", commentID).Scan(&n); err != nil {
		t.Fatalf("counting likes: %v", err)
	}
	return n
}

func TestLikeOnePerAccount(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()
	rep := r.open(t)
	c := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice})
	side := cmsSideRows(t, r)

	st, err := r.svc.Like(ctx, c, r.carol)
	if err != nil || st.Likes != 1 || !st.ViewerLiked {
		t.Fatalf("first Like = %+v, %v, want 1 like and liked", st, err)
	}
	st, err = r.svc.Like(ctx, c, r.carol)
	if err != nil || st.Likes != 1 || !st.ViewerLiked {
		t.Errorf("second Like by the same account = %+v, %v, want unchanged at 1", st, err)
	}
	if n := r.likeRows(t, c); n != 1 {
		t.Errorf("like rows = %d, want 1", n)
	}

	st, err = r.svc.Like(ctx, c, r.bob)
	if err != nil || st.Likes != 2 || !st.ViewerLiked {
		t.Errorf("Like by a second account = %+v, %v, want 2", st, err)
	}
	st, err = r.svc.Like(ctx, c, r.alice)
	if err != nil || st.Likes != 3 {
		t.Errorf("liking your own comment = %+v, %v, want it allowed (3 likes)", st, err)
	}

	st, err = r.svc.Unlike(ctx, c, r.carol)
	if err != nil || st.Likes != 2 || st.ViewerLiked {
		t.Errorf("Unlike = %+v, %v, want 2 likes and not liked", st, err)
	}
	st, err = r.svc.Unlike(ctx, c, r.carol)
	if err != nil || st.Likes != 2 || st.ViewerLiked {
		t.Errorf("second Unlike = %+v, %v, want unchanged at 2", st, err)
	}

	l := r.list(t, rep, r.bob)
	if len(l.Comments) != 1 || l.Comments[0].Likes != 2 || !l.Comments[0].ViewerLiked {
		t.Errorf("listed likes = %+v, want 2 and liked by bob", l.Comments)
	}

	if got := cmsSideRows(t, r); got != side {
		t.Errorf("update and outbox rows %d -> %d: likes must write none (D-16)", side, got)
	}
	if r.note.calls != 0 {
		t.Errorf("notifier calls = %d, want 0 for likes", r.note.calls)
	}
}

// cmsTrustFiles are the files of internal/service that build trust, currency,
// reliability, visibility or the confirmed membership. None may mention a
// comment. The two query files are read the same way, comment lines removed.
var (
	cmsTrustGoFiles = []string{
		"trust.go", "trustfacts.go", "trustview.go", "trustcopy.go", "visibility.go",
		"confirmed.go", "reliability.go", "reliablefilter.go", "currency.go",
	}
	cmsTrustSQLFiles = []string{"../store/queries/votes.sql", "../store/queries/reliability.sql"}
)

func TestLikesNeverFeedTrust(t *testing.T) {
	t.Run("no trust file names a comment", func(t *testing.T) {
		fset := token.NewFileSet()
		for _, name := range cmsTrustGoFiles {
			f, err := parser.ParseFile(fset, name, nil, 0) // no comments are kept
			if err != nil {
				t.Fatalf("parsing %s: %v", name, err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.Ident:
					if strings.Contains(strings.ToLower(x.Name), "comment") {
						t.Errorf("%s: identifier %q mentions a comment", name, x.Name)
					}
				case *ast.BasicLit:
					if x.Kind == token.STRING && strings.Contains(strings.ToLower(x.Value), "comment") {
						t.Errorf("%s: string %s mentions a comment", name, x.Value)
					}
				}
				return true
			})
		}
		for _, name := range cmsTrustSQLFiles {
			raw, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("reading %s: %v", name, err)
			}
			for i, line := range strings.Split(string(raw), "\n") {
				if cut := strings.Index(line, "--"); cut >= 0 {
					line = line[:cut]
				}
				if strings.Contains(strings.ToLower(line), "comment") {
					t.Errorf("%s:%d mentions a comment: %s", name, i+1, strings.TrimSpace(line))
				}
			}
		}
	})

	t.Run("ten likes and a flag leave the trust output byte equal", func(t *testing.T) {
		r := newCmsRig(t)
		ctx := context.Background()
		rep := r.open(t)

		// Two confirmations from two places and a dispute, so the output is not trivial.
		for i, v := range []struct{ value, cell string }{{"confirm", "tdr1v8y"}, {"confirm", "tdr1vc3"}, {"dispute", "tdr1vc9"}} {
			acct := testutil.SeedReporterAccount(t, r.pool, "cms-trust-voter-"+string(rune('a'+i))+"@example.com")
			testutil.MustExec(t, r.pool,
				`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
				 VALUES ($1, $2, 'content', $3, $4, $5)`, rep, acct, v.value, v.cell, r.now.Add(-time.Hour))
		}

		c := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice})
		reply := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.carol, ParentID: c})

		trustJSON := func() (string, []sqlcgen.CurrentVotesForReportsRow) {
			rc, err := r.q.ReportVoteContext(ctx, rep)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := r.q.CurrentVotesForReports(ctx, []int64{rep})
			if err != nil {
				t.Fatal(err)
			}
			meta := service.ReportMeta{
				Severity:   service.Severity(rc.Severity),
				Category:   service.Category(rc.Category),
				Moderation: service.ModerationState(rc.ModerationState),
			}
			tally := service.BuildVoteTally(rows, rep, rc.ReporterAccountID)
			vis, _ := service.Resolve(meta, tally, r.now)
			tr := service.BuildTrust(service.TrustInput{
				Rows: rows, ReportID: rep, ReporterAccountID: rc.ReporterAccountID, Meta: meta,
				Visibility: vis, CreatedAt: rc.CreatedAt, ExpiresAt: rc.ExpiresAt, Now: r.now,
			})
			raw, err := json.Marshal(tr)
			if err != nil {
				t.Fatal(err)
			}
			return string(raw) + "|" + string(vis), rows
		}

		before, rowsBefore := trustJSON()
		if len(rowsBefore) != 3 || len(before) < 40 {
			t.Fatalf("setup: %d vote rows, output %q; want a real output over 3 votes", len(rowsBefore), before)
		}

		for i := 0; i < 10; i++ {
			acct := testutil.SeedReporterAccount(t, r.pool, "cms-liker-"+string(rune('a'+i))+"@example.com")
			if _, err := r.svc.Like(ctx, c, acct); err != nil {
				t.Fatalf("Like %d = %v", i, err)
			}
		}
		if err := r.svc.Flag(ctx, c, r.bob, service.FlagInput{Reasons: []service.FlagReason{service.FlagSpam}}); err != nil {
			t.Fatalf("Flag = %v", err)
		}
		if _, err := r.svc.Like(ctx, reply, r.alice); err != nil {
			t.Fatalf("Like reply = %v", err)
		}

		after, rowsAfter := trustJSON()
		if after != before {
			t.Errorf("trust output changed after likes and a flag:\nbefore %s\nafter  %s", before, after)
		}
		if !reflect.DeepEqual(rowsBefore, rowsAfter) {
			t.Errorf("the vote rows changed after likes and a flag")
		}
	})
}
