package public

import (
	"net/http"

	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
)

func (h *Handler) authVerifyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := common.UserFromContext(r.Context())
		if !ok {
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "認証情報の取得に失敗しました"})
			return
		}

		common.WriteJSON(h.logger, w, http.StatusOK, map[string]any{
			"status": "ok",
			"user":   user,
		})
	}
}
