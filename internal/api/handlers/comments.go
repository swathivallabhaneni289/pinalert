// comments.go is the HTTP surface of comments (Phase 8 plan 12, PAGE-02, SC3).
// Six gated routes decode the request, call service.CommentService and encode
// the answer; every rule lives in the service. Posts and replies are limited by
// RateGate.CommentMiddleware, likes, unlikes and deletes by LikeMiddleware and
// a comment flag by FlagMiddleware. A cross site POST carries no session cookie
// (DEC-L, SameSite=Lax), so no CSRF token is needed.
//
// The wire carries flags the server wrote (is_own, is_reporter, can_delete,
// viewer_liked, viewer_flagged, comments_open) and a username or a placeholder
// name, and never an account id, email, session or cell (A-P8-10). A handler
// logs ids and outcome words only, never a comment's words or a username.

package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// maxCommentBodyBytes caps a comment POST body. A comment is at most 280 code
// points, which is under 1.2 KiB even when every one is a four byte character
// or a six byte JSON escape, so 8 KiB is generous and still small.
const maxCommentBodyBytes = 8192

// commentReportGoneMessage answers a comment action on a report that is not
// available: unknown, withheld or too old to list.
const commentReportGoneMessage = "This report is no longer available."

// PostCommentRequest is the body of POST /reports/{id}/comments. ReplyTo is the
// id of the comment answered, or absent for a top level comment.
type PostCommentRequest struct {
	Words   string `json:"words" maxLength:"280" example:"The road is open again from the north side."`
	ReplyTo *int64 `json:"reply_to,omitempty" example:"41"`
}

// CommentResponse is one comment as the reader sees it. It holds no account id,
// email, session or cell. A deleted or removed comment that still holds visible
// replies is a placeholder: deleted or removed is true and words, author and
// likes are empty.
type CommentResponse struct {
	ID            int64             `json:"id" example:"41"`
	Author        string            `json:"author" example:"asha_k"`
	AuthorKind    string            `json:"author_kind" enums:"user,new_member,deleted_account" example:"user"`
	ReplyToAuthor string            `json:"reply_to_author,omitempty" example:"ravi_m"`
	IsOwn         bool              `json:"is_own" example:"false"`
	IsReporter    bool              `json:"is_reporter" example:"false"`
	CanDelete     bool              `json:"can_delete" example:"false"`
	ViewerLiked   bool              `json:"viewer_liked" example:"false"`
	ViewerFlagged bool              `json:"viewer_flagged" example:"false"`
	Likes         int               `json:"likes" example:"3"`
	ReplyCount    int               `json:"reply_count" example:"1"`
	At            string            `json:"at" example:"2026-10-29T10:15:00Z"`
	Deleted       bool              `json:"deleted" example:"false"`
	Removed       bool              `json:"removed" example:"false"`
	Held          bool              `json:"held" example:"false"`
	Words         string            `json:"words" example:"The road is open again from the north side."`
	Replies       []CommentResponse `json:"replies"`
}

// CommentListResponse is one page of a report's comments. CommentCount is the
// number everyone can see, not the page length. NextAfter is the id to pass as
// the after query value for the next page.
type CommentListResponse struct {
	Comments     []CommentResponse `json:"comments"`
	CommentCount int               `json:"comment_count" example:"12"`
	CommentsOpen bool              `json:"comments_open" example:"true"`
	More         bool              `json:"more" example:"false"`
	NextAfter    int64             `json:"next_after" example:"0"`
}

// LikeResponse is the answer to a like or an unlike: the fresh count and
// whether the caller now likes the comment.
type LikeResponse struct {
	Likes       int  `json:"likes" example:"4"`
	ViewerLiked bool `json:"viewer_liked" example:"true"`
}

// DeleteCommentResponse is the answer to a delete of your own comment.
type DeleteCommentResponse struct {
	Deleted bool `json:"deleted" example:"true"`
}

func commentResponse(v service.CommentView) CommentResponse {
	out := CommentResponse{
		ID:            v.ID,
		Author:        v.Author,
		AuthorKind:    v.AuthorKind,
		ReplyToAuthor: v.ReplyToAuthor,
		IsOwn:         v.IsOwn,
		IsReporter:    v.IsReporter,
		CanDelete:     v.CanDelete,
		ViewerLiked:   v.ViewerLiked,
		ViewerFlagged: v.ViewerFlagged,
		Likes:         v.Likes,
		ReplyCount:    v.ReplyCount,
		At:            v.At.UTC().Format(time.RFC3339),
		Deleted:       v.Deleted,
		Removed:       v.Removed,
		Held:          v.Held,
		Words:         v.Words,
		Replies:       make([]CommentResponse, 0, len(v.Replies)),
	}
	for _, rep := range v.Replies {
		out.Replies = append(out.Replies, commentResponse(rep))
	}
	return out
}

// parseCommentID reads the {id} path value of a comment route.
func parseCommentID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeFieldError(w, http.StatusBadRequest, "id", "Comment id must be a number.")
		return 0, false
	}
	return id, true
}

// commentAccount returns the caller, or writes the 500 of a route mounted
// outside the gated group, a programming error.
func commentAccount(w http.ResponseWriter, r *http.Request) (account.Account, bool) {
	acc, ok := account.FromContext(r.Context())
	if !ok {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return account.Account{}, false
	}
	return acc, true
}

// writeCommentError maps the sentinel errors every comment route can meet to
// their status and words. It returns false for an error it does not know, so
// the caller logs it and answers a plain 500.
func writeCommentError(w http.ResponseWriter, err error) bool {
	var rej service.CommentRejectedError
	var nr service.FlagNoteRejectedError
	var ve service.ValidationError
	switch {
	case errors.Is(err, service.ErrReportNotFound):
		writeFieldError(w, http.StatusNotFound, "report", commentReportGoneMessage)
	case errors.Is(err, service.ErrCommentGone):
		writeFieldError(w, http.StatusNotFound, "comment", service.CommentGoneMessage)
	case errors.Is(err, service.ErrCommentsClosed):
		writeFieldError(w, http.StatusConflict, "comments", service.CommentsClosedMessage)
	case errors.Is(err, service.ErrCommentNeedsUsername):
		writeFieldError(w, http.StatusConflict, "username", service.CommentNeedsUsernameMessage)
	case errors.Is(err, service.ErrCommentNotYours):
		writeFieldError(w, http.StatusForbidden, "comment", service.CommentNotYoursMessage)
	case errors.Is(err, service.ErrCommentOwnFlag):
		writeFieldError(w, http.StatusConflict, "flag", service.CommentOwnFlagMessage)
	case errors.Is(err, service.ErrAlreadyFlagged):
		writeFieldError(w, http.StatusConflict, "flag", "You already flagged this comment. Moderators will see it.")
	case errors.Is(err, service.ErrCommentEmpty):
		writeFieldError(w, http.StatusBadRequest, "words", service.CommentEmptyMessage)
	case errors.Is(err, service.ErrCommentTooLong):
		writeFieldError(w, http.StatusBadRequest, "words", service.CommentTooLongMessage)
	case errors.Is(err, service.ErrCommentCharacters):
		writeFieldError(w, http.StatusBadRequest, "words", service.CommentCharactersMessage)
	case errors.As(err, &rej):
		writeFieldError(w, http.StatusUnprocessableEntity, "words", service.RejectMessage(rej.Reason))
	case errors.As(err, &nr):
		writeFieldError(w, http.StatusUnprocessableEntity, "note", service.RejectMessage(nr.Reason))
	case errors.As(err, &ve):
		writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
	default:
		return false
	}
	return true
}

// ListComments produces the handler of GET /reports/{id}/comments.
//
// @Summary List the comments of a report
// @Description Returns one page of the report's comments, top level comments oldest first, each
// @Description followed by its replies, oldest first. The page holds at most 50 top level
// @Description comments; when more is true pass next_after as the after query value to read the
// @Description next page. comment_count is the number everyone can see. Every flag in the answer
// @Description is written for the caller (is_own, is_reporter, can_delete, viewer_liked,
// @Description viewer_flagged), and comments_open says whether people may still post and like.
// @Description A comment that was deleted or removed but still has visible replies appears as a
// @Description placeholder with no words. No account id, email or session is ever sent. A report
// @Description that does not exist, is not available or ended too long ago gets 404.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags comments
// @Produce json
// @Param id path int true "Report id"
// @Param after query int false "Id of the last top level comment of the previous page"
// @Success 200 {object} CommentListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/comments [get]
func ListComments(svc *service.CommentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		acc, ok := commentAccount(w, r)
		if !ok {
			return
		}
		reportID, ok := parseReportID(w, r)
		if !ok {
			return
		}
		var after int64
		if raw := r.URL.Query().Get("after"); raw != "" {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || n <= 0 {
				writeFieldError(w, http.StatusBadRequest, "after", "The after value must be a positive number.")
				return
			}
			after = n
		}

		list, err := svc.List(r.Context(), reportID, acc.ID, after)
		if err != nil {
			if writeCommentError(w, err) {
				return
			}
			log.Printf("handlers: ListComments: report=%d: %v", reportID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		out := CommentListResponse{
			Comments:     make([]CommentResponse, 0, len(list.Comments)),
			CommentCount: list.Count,
			CommentsOpen: list.Open,
			More:         list.More,
			NextAfter:    list.NextAfter,
		}
		for _, c := range list.Comments {
			out.Comments = append(out.Comments, commentResponse(c))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// PostComment produces the handler of POST /reports/{id}/comments.
//
// @Summary Post a comment or a reply
// @Description Posts a comment of at most 280 letters on a report, or a reply when reply_to names
// @Description a comment of the same report. The words go through the abuse check: a clear or
// @Description borderline comment is posted (201), a refused one gets 422 with the same sentence
// @Description a refused report gets, and a comment whose check could not finish is posted held,
// @Description with held true, and is shown only to its author until it is checked. Comments are
// @Description closed (409, field comments) once the report is resolved or has faded, and an
// @Description account with no username gets 409 with field username. Posting is limited per
// @Description verified account, every attempt counts, and a blocked post gets 429 with a
// @Description Retry-After header; the words stay in the writer's box.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags comments
// @Accept json
// @Produce json
// @Param id path int true "Report id"
// @Param body body PostCommentRequest true "The words, and the comment answered if this is a reply"
// @Success 201 {object} CommentResponse
// @Failure 400 {object} ErrorResponse "Empty, too long or holding characters that cannot be posted"
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse "The report or the comment answered is not available"
// @Failure 409 {object} ErrorResponse "Comments are closed, or the account has no username"
// @Failure 422 {object} ErrorResponse "The words were refused by the abuse check"
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/comments [post]
func PostComment(svc *service.CommentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		acc, ok := commentAccount(w, r)
		if !ok {
			return
		}
		reportID, ok := parseReportID(w, r)
		if !ok {
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxCommentBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req PostCommentRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}
		if req.ReplyTo != nil && *req.ReplyTo <= 0 {
			writeFieldError(w, http.StatusBadRequest, "reply_to", "Reply target must be a number.")
			return
		}

		// The screener call inside Post is bounded at InlineScreenTimeout (3 s),
		// inside the 10 s server write timeout, so no write deadline is extended.
		view, err := svc.Post(r.Context(), service.CommentInput{
			ReportID:    reportID,
			AccountID:   acc.ID,
			HasUsername: acc.HasUsername,
			Words:       req.Words,
			ReplyTo:     req.ReplyTo,
		})
		if err != nil {
			if writeCommentError(w, err) {
				return
			}
			log.Printf("handlers: PostComment: report=%d account=%d: %v", reportID, acc.ID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, commentResponse(view))
	}
}

// LikeComment produces the handler of POST /comments/{id}/like.
//
// @Summary Like a comment
// @Description Adds the caller's like to a comment and answers the fresh count. One like counts per
// @Description account, so a repeat changes nothing. A like never changes any trust number. Likes
// @Description are closed with the comments (409, field comments); a comment that is deleted,
// @Description removed or held back, or on a report that is not available, gets 404. Liking is
// @Description limited per verified account, and a blocked like gets 429 with a Retry-After header.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags comments
// @Produce json
// @Param id path int true "Comment id"
// @Success 200 {object} LikeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "Comments are closed"
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /comments/{id}/like [post]
func LikeComment(svc *service.CommentService) http.HandlerFunc {
	return likeHandler(svc, true)
}

// UnlikeComment produces the handler of POST /comments/{id}/unlike.
//
// @Summary Take back a like on a comment
// @Description Removes the caller's like from a comment and answers the fresh count. Taking back a
// @Description like that was never given changes nothing. The same closed, gone and limit answers
// @Description as a like apply.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags comments
// @Produce json
// @Param id path int true "Comment id"
// @Success 200 {object} LikeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "Comments are closed"
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /comments/{id}/unlike [post]
func UnlikeComment(svc *service.CommentService) http.HandlerFunc {
	return likeHandler(svc, false)
}

func likeHandler(svc *service.CommentService, like bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		acc, ok := commentAccount(w, r)
		if !ok {
			return
		}
		commentID, ok := parseCommentID(w, r)
		if !ok {
			return
		}
		var (
			st  service.LikeState
			err error
		)
		if like {
			st, err = svc.Like(r.Context(), commentID, acc.ID)
		} else {
			st, err = svc.Unlike(r.Context(), commentID, acc.ID)
		}
		if err != nil {
			if writeCommentError(w, err) {
				return
			}
			log.Printf("handlers: likeComment: comment=%d like=%t: %v", commentID, like, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, LikeResponse{Likes: st.Likes, ViewerLiked: st.ViewerLiked})
	}
}

// DeleteComment produces the handler of POST /comments/{id}/delete.
//
// @Summary Delete your own comment
// @Description Deletes the caller's own comment, at any time and whatever the state of the report.
// @Description There is no edit. A comment that has visible replies stays as a placeholder with no
// @Description words. Deleting a comment that is already deleted answers 200 again. Someone else's
// @Description comment gets 403, and an unknown comment gets 404. Deleting is limited with likes,
// @Description and a blocked delete gets 429 with a Retry-After header.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags comments
// @Produce json
// @Param id path int true "Comment id"
// @Success 200 {object} DeleteCommentResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "The comment belongs to someone else"
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /comments/{id}/delete [post]
func DeleteComment(svc *service.CommentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		acc, ok := commentAccount(w, r)
		if !ok {
			return
		}
		commentID, ok := parseCommentID(w, r)
		if !ok {
			return
		}
		if err := svc.Delete(r.Context(), commentID, acc.ID); err != nil {
			if writeCommentError(w, err) {
				return
			}
			log.Printf("handlers: DeleteComment: comment=%d: %v", commentID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, DeleteCommentResponse{Deleted: true})
	}
}

// FlagComment produces the handler of POST /comments/{id}/flag.
//
// @Summary Flag a comment for the moderators
// @Description Sends a comment to the moderators with one or more of five reasons: spam, abusive,
// @Description false, dangerous or other, each at most once, and an optional note of at most 300
// @Description characters that only moderators see. The note goes through the same text check as a
// @Description report flag note, and a refused note gets 422. A flag never hides or changes the
// @Description comment. Each account can flag a comment once, and a repeat gets 409; your own
// @Description comment also gets 409. Flagging stays open after the comments close. It shares the
// @Description report flag limit, and a blocked flag gets 429 with a Retry-After header.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags comments
// @Accept json
// @Produce json
// @Param id path int true "Comment id"
// @Param body body FlagRequest true "One or more of the five reasons and an optional note"
// @Success 201 {object} FlagResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "Your own comment, or already flagged by you"
// @Failure 422 {object} ErrorResponse "The note was refused by the text check"
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /comments/{id}/flag [post]
func FlagComment(svc *service.CommentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		acc, ok := commentAccount(w, r)
		if !ok {
			return
		}
		commentID, ok := parseCommentID(w, r)
		if !ok {
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxFlagBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req FlagRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}
		if req.Reason != "" && req.Reasons != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}
		raw := req.Reasons
		if req.Reason != "" {
			raw = []string{req.Reason}
		}
		reasons := make([]service.FlagReason, len(raw))
		for i, s := range raw {
			reasons[i] = service.FlagReason(s)
		}

		err := svc.Flag(r.Context(), commentID, acc.ID, service.FlagInput{
			AccountID: acc.ID,
			Reasons:   reasons,
			Note:      req.Note,
		})
		if err != nil {
			if writeCommentError(w, err) {
				return
			}
			log.Printf("handlers: FlagComment: comment=%d: %v", commentID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, FlagResponse{Flagged: true})
	}
}
