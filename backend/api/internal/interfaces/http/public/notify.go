package public

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
)

func (h *Handler) notifyReviewReceipt(ctx context.Context, user common.AuthenticatedUser, summary reviewSummaryResponse, comment string) {
	if ctx == nil {
		ctx = context.Background()
	}

	if userID := strings.TrimSpace(user.ID); userID != "" {
		message := buildReceiptMessage(summary, comment)
		if err := h.sendLineMessage(ctx, userID, message); err != nil && h.logger != nil {
			h.logger.Printf("LINE通知の送信に失敗: %v", err)
		}
	}

	if strings.TrimSpace(h.discordDestination) != "" {
		discordMessage := buildDiscordReviewMessage(h.adminReviewBaseURL, user, summary, comment)
		if discordMessage != "" {
			identifier := summary.ID
			if identifier == "" {
				identifier = strings.TrimSpace(user.Username)
			}
			if identifier == "" {
				identifier = user.ID
			}
			if identifier == "" {
				identifier = "discord"
			}
			if err := h.sendDiscordMessage(ctx, identifier, discordMessage); err != nil && h.logger != nil {
				h.logger.Printf("Discord通知の送信に失敗: %v", err)
			}
		}
	}
}

func buildReceiptMessage(summary reviewSummaryResponse, comment string) string {
	sections := [][]string{}

	addSection := func(title, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		sections = append(sections, []string{
			fmt.Sprintf("**%s**", title),
			"> " + value,
		})
	}

	addSection("店舗名", summary.StoreName)
	addSection("支店名", summary.BranchName)
	addSection("都道府県", summary.Prefecture)
	addSection("訪問時期", formatVisitedDisplay(summary.VisitedAt))
	if len(summary.Industries) > 0 {
		addSection("業種", strings.Join(summary.Industries, " / "))
	}
	if summary.Age > 0 {
		addSection("年齢", fmt.Sprintf("%d歳", summary.Age))
	}
	addSection("コメント", comment)

	var builder strings.Builder
	builder.WriteString("アンケートのご協力ありがとうございます！\n")
	for _, section := range sections {
		builder.WriteString(section[0])
		builder.WriteString("\n")
		builder.WriteString(section[1])
		builder.WriteString("\n")
	}
	return builder.String()
}

func buildDiscordReviewMessage(adminBaseURL string, user common.AuthenticatedUser, summary reviewSummaryResponse, comment string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**%s** から新しいアンケート投稿があります。\n", reviewerDisplayName(user)))
	builder.WriteString(fmt.Sprintf("- 店舗: %s (%s)\n", summary.StoreName, summary.Prefecture))
	builder.WriteString(fmt.Sprintf("- 訪問: %s\n", formatVisitedDisplay(summary.VisitedAt)))
	builder.WriteString(fmt.Sprintf("- 総評: %.1f / 5\n", summary.Rating))
	builder.WriteString(fmt.Sprintf("- コメント: %s\n", comment))
	if summary.ID != "" && strings.TrimSpace(adminBaseURL) != "" {
		builder.WriteString(fmt.Sprintf("[管理画面で確認](%s/%s)\n", strings.TrimRight(adminBaseURL, "/"), summary.ID))
	}
	return builder.String()
}

func (h *Handler) sendLineMessage(ctx context.Context, userID, text string) error {
	return h.sendMessengerMessage(ctx, h.messengerDestination, userID, text)
}

func (h *Handler) sendDiscordMessage(ctx context.Context, userID, text string) error {
	dest := strings.TrimSpace(h.discordDestination)
	if dest == "" {
		return nil
	}
	return h.sendMessengerMessage(ctx, dest, userID, text)
}

func (h *Handler) sendMessengerMessage(ctx context.Context, destination, userID, bodyText string) error {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return errors.New("userID is required")
	}

	payload := map[string]any{
		"userId": trimmedUserID,
		"text":   bodyText,
	}
	if dest := strings.TrimSpace(destination); dest != "" {
		payload["destination"] = dest
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("メッセンジャー送信用ペイロードの作成に失敗: %w", err)
	}

	timeout := h.httpClient.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpoint := strings.TrimRight(h.messengerEndpoint, "/") + "/messages"
	req, err := http.NewRequestWithContext(ctxWithTimeout, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("メッセンジャー送信リクエストの作成に失敗: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("メッセンジャー送信リクエストに失敗: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		message, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
		return fmt.Errorf("メッセンジャー送信でエラーが発生: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(message)))
	}

	return nil
}
