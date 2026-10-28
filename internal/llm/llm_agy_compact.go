package llm

import "context"

const agySummaryPrompt = "Tóm tắt cuộc trò chuyện tới giờ trong tối đa 150 từ: " +
	"yêu cầu chính của người dùng, thông tin quan trọng và việc còn dang dở. " +
	"Chỉ trả về bản tóm tắt."

func (p *agyPool) compact(conversation *agyConversation) {
	conversation.mutex.Lock()
	defer conversation.mutex.Unlock()
	if conversation.process == nil || !conversation.needsCompaction() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), agyBackgroundTimeout)
	defer cancel()
	summary, err := conversation.process.ask(ctx, agySummaryPrompt)
	conversation.drop()
	if err != nil {
		return
	}

	conversation.carryOver = summary.Response
	if process, err := p.take(); err == nil {
		conversation.process = process
	}
}

func (m *Manager) PrewarmAgent() {
	if m.agy != nil {
		m.agy.spawn(m.agy.Prewarm)
	}
}

func (m *Manager) restartAgent() {
	if m.agy != nil {
		m.agy.reset()
		m.agy.spawn(m.agy.Prewarm)
	}
}
