// comment_block.go is the route behind the Block button beside Flag on every
// comment written by someone else (quick 261030-u78, WR-E04, D-12). The caller
// names a comment, never a person, so the route cannot be used to probe a
// username or whether a profile is private. It answers 204 on success (also
// when the block already exists, and also when the author has blocked the
// caller) and one 404 body, the same bytes as a like on a deleted comment, for
// every refusal: an unknown comment, one on a report the caller cannot see, a
// finished Rescue report, a deleted, removed or held comment, the caller's own
// comment, one by a deleted account and one by someone shown as New member
// (review CR-01: Block is offered only where the name is shown). It spends the per account people action
// bucket, like a profile block. A cross site POST carries no session cookie
// (DEC-L), so there is no CSRF token.
package handlers

import (
	"errors"
	"log"
	"net/http"

	"pinalert/internal/service"
)

// writeCommentBlockGone is the one refusal of the route.
func writeCommentBlockGone(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	writeFieldError(w, http.StatusNotFound, "comment", service.CommentGoneMessage)
}

// BlockCommentAuthor produces the handler of POST /comments/{id}/block-author.
//
// @Summary Block the author of a comment
// @Description Blocks the person who wrote the comment, whether or not their profile is public. Every
// @Description follow between you ends and neither can follow the other. You stop seeing their comments.
// @Description They are not told. Blocking someone already blocked answers 204 again. Your own comment, an
// @Description unknown comment, one that is deleted, removed, held back, on a report that is not available,
// @Description by a deleted account, or by someone shown as New member all answer the same 404. Send no
// @Description body or an empty object. It is limited together with the other people actions, and a
// @Description blocked call gets 429 with a Retry-After header.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags comments
// @Accept json
// @Produce json
// @Param id path int true "Comment id"
// @Success 204 "Blocked"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /comments/{id}/block-author [post]
func BlockCommentAuthor(people *service.PeopleService) http.HandlerFunc {
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
		if !decodeEmptyBody(w, r) {
			return
		}
		if err := people.BlockCommentAuthor(r.Context(), acc.ID, commentID); err != nil {
			if errors.Is(err, service.ErrCommentGone) {
				writeCommentBlockGone(w)
				return
			}
			log.Printf("handlers: BlockCommentAuthor: comment=%d: %v", commentID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
