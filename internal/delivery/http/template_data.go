package httpdelivery

import (
	"context"
	"fmt"

	"github.com/adnope/ephemeral/internal/domain"
)

type itemTemplateData struct {
	*domain.Item
	PublicLinkActive bool
}

func (h *Handler) itemTemplateData(ctx context.Context, items []*domain.Item) ([]itemTemplateData, error) {
	activeIDs, err := h.items.ActivePublicLinkItemIDs(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("active public link items: %w", err)
	}

	data := make([]itemTemplateData, 0, len(items))
	for _, item := range items {
		data = append(data, itemTemplateData{
			Item:             item,
			PublicLinkActive: item != nil && activeIDs[item.ID],
		})
	}
	return data, nil
}

func singleItemTemplateData(item *domain.Item) itemTemplateData {
	return itemTemplateData{Item: item}
}
