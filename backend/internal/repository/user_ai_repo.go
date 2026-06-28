package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userAIRepository struct {
	sql sqlExecutor
}

func NewUserAIRepository(sqlDB *sql.DB) service.UserAIRepository {
	return &userAIRepository{sql: sqlDB}
}

func (r *userAIRepository) ListChatConversations(ctx context.Context, userID int64, params pagination.PaginationParams, messagesLimit int) (result []service.ChatConversation, paginationResult *pagination.PaginationResult, err error) {
	var total int64
	if err := scanSingleRow(ctx, r.sql, `
		SELECT COUNT(*)
		FROM chat_conversations
		WHERE user_id = $1 AND deleted_at IS NULL
	`, []any{userID}, &total); err != nil {
		return nil, nil, err
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, user_id, group_id, title, title_generated, model, created_at, updated_at
		FROM chat_conversations
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY updated_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	conversations := make([]service.ChatConversation, 0, params.Limit())
	for rows.Next() {
		conversation, scanErr := scanChatConversation(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		conversations = append(conversations, conversation)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	for i := range conversations {
		messages, err := r.listMessages(ctx, userID, conversations[i].ID, messagesLimit)
		if err != nil {
			return nil, nil, err
		}
		conversations[i].Messages = messages
	}

	return conversations, paginationResultFromTotal(total, params), nil
}

func (r *userAIRepository) GetChatConversation(ctx context.Context, userID, conversationID int64) (*service.ChatConversation, error) {
	var conversation service.ChatConversation
	if err := scanSingleRow(ctx, r.sql, `
		SELECT id, user_id, group_id, title, title_generated, model, created_at, updated_at
		FROM chat_conversations
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, []any{conversationID, userID},
		&conversation.ID,
		&conversation.UserID,
		&conversation.GroupID,
		&conversation.Title,
		&conversation.TitleGenerated,
		&conversation.Model,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAIConversationNotFound
		}
		return nil, err
	}
	messages, err := r.listMessages(ctx, userID, conversationID, 100)
	if err != nil {
		return nil, err
	}
	conversation.Messages = messages
	return &conversation, nil
}

func (r *userAIRepository) CreateChatConversation(ctx context.Context, input service.ChatConversationCreateInput) (*service.ChatConversation, error) {
	var conversation service.ChatConversation
	if err := scanSingleRow(ctx, r.sql, `
		INSERT INTO chat_conversations (user_id, group_id, title, model)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, group_id, title, title_generated, model, created_at, updated_at
	`, []any{input.UserID, input.GroupID, input.Title, input.Model},
		&conversation.ID,
		&conversation.UserID,
		&conversation.GroupID,
		&conversation.Title,
		&conversation.TitleGenerated,
		&conversation.Model,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (r *userAIRepository) UpdateChatConversationTitle(ctx context.Context, input service.ChatConversationTitleUpdateInput) (*service.ChatConversation, error) {
	var conversation service.ChatConversation
	if err := scanSingleRow(ctx, r.sql, `
		UPDATE chat_conversations
		SET title = $3, title_generated = TRUE
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL AND title_generated = FALSE
		RETURNING id, user_id, group_id, title, title_generated, model, created_at, updated_at
	`, []any{input.ConversationID, input.UserID, input.Title},
		&conversation.ID,
		&conversation.UserID,
		&conversation.GroupID,
		&conversation.Title,
		&conversation.TitleGenerated,
		&conversation.Model,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return r.GetChatConversation(ctx, input.UserID, input.ConversationID)
		}
		return nil, err
	}

	messages, err := r.listMessages(ctx, input.UserID, input.ConversationID, 100)
	if err != nil {
		return nil, err
	}
	conversation.Messages = messages
	return &conversation, nil
}

func (r *userAIRepository) DeleteChatConversation(ctx context.Context, userID, conversationID int64) error {
	res, err := r.sql.ExecContext(ctx, `
		UPDATE chat_conversations
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, conversationID, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAIConversationNotFound
	}
	return nil
}

func (r *userAIRepository) CreateChatMessage(ctx context.Context, input service.ChatMessageCreateInput) (*service.ChatMessage, error) {
	var message service.ChatMessage
	if err := scanSingleRow(ctx, r.sql, `
		INSERT INTO chat_messages (conversation_id, user_id, role, content, model)
		SELECT $1, $2, $3, $4, $5
		WHERE EXISTS (
			SELECT 1 FROM chat_conversations
			WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		)
		RETURNING id, conversation_id, user_id, role, content, model, created_at, updated_at
	`, []any{input.ConversationID, input.UserID, input.Role, input.Content, input.Model},
		&message.ID,
		&message.ConversationID,
		&message.UserID,
		&message.Role,
		&message.Content,
		&message.Model,
		&message.CreatedAt,
		&message.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAIConversationNotFound
		}
		return nil, err
	}
	return &message, nil
}

func (r *userAIRepository) UpdateChatConversationAfterMessage(ctx context.Context, userID, conversationID int64, title, model string, groupID *int64) error {
	res, err := r.sql.ExecContext(ctx, `
		UPDATE chat_conversations
		SET
			title = CASE WHEN title = '' THEN $3 ELSE title END,
			model = CASE WHEN $4 <> '' THEN $4 ELSE model END,
			group_id = COALESCE($5, group_id),
			updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, conversationID, userID, title, model, groupID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAIConversationNotFound
	}
	return nil
}

func (r *userAIRepository) CreateImageGenerationHistory(ctx context.Context, input service.ImageGenerationHistoryCreateInput) (*service.ImageGenerationHistory, error) {
	imagesJSON, err := json.Marshal(input.Images)
	if err != nil {
		return nil, fmt.Errorf("marshal image generation results: %w", err)
	}

	var item service.ImageGenerationHistory
	var groupID sql.NullInt64
	var imagesRaw []byte
	if err := scanSingleRow(ctx, r.sql, `
		INSERT INTO image_generation_history (user_id, group_id, prompt, model, size, n, images)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
		RETURNING id, user_id, group_id, prompt, model, size, n, images, created_at
	`, []any{input.UserID, input.GroupID, input.Prompt, input.Model, input.Size, input.N, string(imagesJSON)},
		&item.ID,
		&item.UserID,
		&groupID,
		&item.Prompt,
		&item.Model,
		&item.Size,
		&item.N,
		&imagesRaw,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	if groupID.Valid {
		item.GroupID = &groupID.Int64
	}
	item.Images = decodeImageGenerationImages(imagesRaw)
	return &item, nil
}

func (r *userAIRepository) ListImageGenerationHistory(ctx context.Context, userID int64, params pagination.PaginationParams) (result []service.ImageGenerationHistory, paginationResult *pagination.PaginationResult, err error) {
	var total int64
	if err := scanSingleRow(ctx, r.sql, `
		SELECT COUNT(*)
		FROM image_generation_history
		WHERE user_id = $1
	`, []any{userID}, &total); err != nil {
		return nil, nil, err
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, user_id, group_id, prompt, model, size, n, images, created_at
		FROM image_generation_history
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	items := make([]service.ImageGenerationHistory, 0, params.Limit())
	for rows.Next() {
		item, scanErr := scanImageGenerationHistory(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return items, paginationResultFromTotal(total, params), nil
}

func (r *userAIRepository) listMessages(ctx context.Context, userID, conversationID int64, limit int) (result []service.ChatMessage, err error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, conversation_id, user_id, role, content, model, created_at, updated_at
		FROM (
			SELECT id, conversation_id, user_id, role, content, model, created_at, updated_at
			FROM chat_messages
			WHERE conversation_id = $1 AND user_id = $2
			ORDER BY created_at DESC, id DESC
			LIMIT $3
		) recent
		ORDER BY created_at ASC, id ASC
	`, conversationID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	messages := make([]service.ChatMessage, 0, limit)
	for rows.Next() {
		message, scanErr := scanChatMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

type chatConversationScanner interface {
	Scan(dest ...any) error
}

func scanChatConversation(scanner chatConversationScanner) (service.ChatConversation, error) {
	var conversation service.ChatConversation
	var groupID sql.NullInt64
	var createdAt, updatedAt time.Time
	if err := scanner.Scan(
		&conversation.ID,
		&conversation.UserID,
		&groupID,
		&conversation.Title,
		&conversation.TitleGenerated,
		&conversation.Model,
		&createdAt,
		&updatedAt,
	); err != nil {
		return service.ChatConversation{}, err
	}
	if groupID.Valid {
		conversation.GroupID = &groupID.Int64
	}
	conversation.CreatedAt = createdAt
	conversation.UpdatedAt = updatedAt
	return conversation, nil
}

func scanChatMessage(scanner chatConversationScanner) (service.ChatMessage, error) {
	var message service.ChatMessage
	if err := scanner.Scan(
		&message.ID,
		&message.ConversationID,
		&message.UserID,
		&message.Role,
		&message.Content,
		&message.Model,
		&message.CreatedAt,
		&message.UpdatedAt,
	); err != nil {
		return service.ChatMessage{}, fmt.Errorf("scan chat message: %w", err)
	}
	return message, nil
}

func scanImageGenerationHistory(scanner chatConversationScanner) (service.ImageGenerationHistory, error) {
	var item service.ImageGenerationHistory
	var groupID sql.NullInt64
	var imagesRaw []byte
	if err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&groupID,
		&item.Prompt,
		&item.Model,
		&item.Size,
		&item.N,
		&imagesRaw,
		&item.CreatedAt,
	); err != nil {
		return service.ImageGenerationHistory{}, fmt.Errorf("scan image generation history: %w", err)
	}
	if groupID.Valid {
		item.GroupID = &groupID.Int64
	}
	item.Images = decodeImageGenerationImages(imagesRaw)
	return item, nil
}

func decodeImageGenerationImages(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var images []string
	if err := json.Unmarshal(raw, &images); err != nil {
		return []string{}
	}
	if images == nil {
		return []string{}
	}
	return images
}
