package graph

import (
	"github.com/hellohirusha/ownstall/graph/model"
	"github.com/hellohirusha/ownstall/internal/services"
)

// ticketToGraphQL maps a service-layer ticket onto the generated GraphQL type.
func ticketToGraphQL(t *services.Ticket) *model.Ticket {
	out := &model.Ticket{
		ID:                 parseUUID(t.ID),
		Number:             int32(t.Number),
		Subject:            t.Subject,
		Status:             t.Status,
		Priority:           t.Priority,
		CustomerEmail:      t.CustomerEmail,
		CustomerName:       t.CustomerName,
		AssigneeName:       t.AssigneeName,
		Source:             t.Source,
		SLAStatus:          t.SLAStatus,
		SLAFirstResponseAt: t.SLAFirstResponseAt,
		FirstResponseAt:    t.FirstResponseAt,
		ResolvedAt:         t.ResolvedAt,
		AiDraftBody:        t.AIDraftBody,
		AiDraftConfidence:  t.AIDraftConfidence,
		LatestMessage:      t.LatestMessage,
		UnreadCount:        int32(t.UnreadCount),
		Messages:           []*model.TicketMessage{},
		CreatedAt:          t.CreatedAt,
		UpdatedAt:          t.UpdatedAt,
	}
	if t.AssigneeID != nil {
		aid := parseUUID(*t.AssigneeID)
		out.AssigneeID = &aid
	}
	for i := range t.Messages {
		out.Messages = append(out.Messages, ticketMessageToGraphQL(&t.Messages[i]))
	}
	return out
}

// ticketMessageToGraphQL maps a service-layer message onto the generated GraphQL type.
func ticketMessageToGraphQL(m *services.Message) *model.TicketMessage {
	return &model.TicketMessage{
		ID:          parseUUID(m.ID),
		AuthorType:  m.AuthorType,
		AuthorName:  m.AuthorName,
		AuthorEmail: m.AuthorEmail,
		Body:        m.Body,
		IsInternal:  m.IsInternal,
		CreatedAt:   m.CreatedAt,
	}
}
