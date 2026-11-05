package chatbot

import api "billionmail-core/api/chatbot"

type ControllerV1 struct{}

func NewV1() api.IChatbotV1 { return &ControllerV1{} }
