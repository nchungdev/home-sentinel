package llm

import (
	"context"
	"sync"
	"time"
)

const (
	agyCompactContextTokens = 40000
	agyMaxTurns             = 30
	agyIdleTimeout          = 10 * time.Minute
	agyMaxConversations     = 3
	agyBackgroundTimeout    = 90 * time.Second
	agyDefaultConversation  = "default"
	agyPrimePrompt          = "Chỉ trả lời: OK"
)

type conversationKey struct{}

func WithConversation(ctx context.Context, conversationID string) context.Context {
	return context.WithValue(ctx, conversationKey{}, conversationID)
}

func conversationFrom(ctx context.Context) string {
	if conversationID, _ := ctx.Value(conversationKey{}).(string); conversationID != "" {
		return conversationID
	}
	return agyDefaultConversation
}

type agyConversation struct {
	mutex     sync.Mutex
	process   *agyProcess
	lastUsed  time.Time
	carryOver string
}

type agyPool struct {
	manager       *Manager
	mutex         sync.Mutex
	conversations map[string]*agyConversation
	spare         *agyProcess
	refilling     bool
	generation    int
	background    sync.WaitGroup
}

func newAgyPool(manager *Manager) *agyPool {
	return &agyPool{manager: manager, conversations: make(map[string]*agyConversation)}
}

func (p *agyPool) ask(
	ctx context.Context, conversationID, prompt string, onChunk func(string),
) error {
	conversation := p.conversation(conversationID)
	conversation.mutex.Lock()
	defer conversation.mutex.Unlock()

	if conversation.process != nil && !conversation.reusable() {
		conversation.drop()
	}
	if conversation.process == nil {
		process, err := p.take()
		if err != nil {
			return err
		}
		conversation.process = process
	}

	result, err := conversation.process.ask(ctx, conversation.turnText(prompt))
	if err != nil {
		conversation.drop()
		return err
	}
	conversation.lastUsed = time.Now()
	onChunk(result.Response)

	if conversation.needsCompaction() {
		p.spawn(func() { p.compact(conversation) })
	}
	return nil
}

func (c *agyConversation) reusable() bool {
	return c.process.alive() && time.Since(c.lastUsed) < agyIdleTimeout
}

func (c *agyConversation) needsCompaction() bool {
	return c.process.contextTokens >= agyCompactContextTokens || c.process.turns >= agyMaxTurns
}

func (c *agyConversation) turnText(prompt string) string {
	text := prompt
	if c.carryOver != "" {
		text = "Tóm tắt cuộc trò chuyện trước đó:\n" + c.carryOver + "\n\n" + prompt
		c.carryOver = ""
	}
	if c.process.turns == 0 {
		text = SystemPrompt + "\n\n" + text
	}
	return text
}

func (c *agyConversation) drop() {
	if c.process != nil {
		c.process.close()
		c.process = nil
	}
}

func (p *agyPool) conversation(conversationID string) *agyConversation {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if conversation := p.conversations[conversationID]; conversation != nil {
		return conversation
	}
	if len(p.conversations) >= agyMaxConversations {
		p.evictOldestLocked()
	}
	conversation := &agyConversation{lastUsed: time.Now()}
	p.conversations[conversationID] = conversation
	return conversation
}

func (p *agyPool) evictOldestLocked() {
	oldestID := ""
	var oldest *agyConversation
	for conversationID, conversation := range p.conversations {
		if oldest == nil || conversation.lastUsed.Before(oldest.lastUsed) {
			oldestID, oldest = conversationID, conversation
		}
	}
	delete(p.conversations, oldestID)
	p.spawn(func() {
		oldest.mutex.Lock()
		defer oldest.mutex.Unlock()
		oldest.drop()
	})
}

func (p *agyPool) take() (*agyProcess, error) {
	p.mutex.Lock()
	spare := p.spare
	p.spare = nil
	p.mutex.Unlock()
	p.spawn(p.Prewarm)

	if spare != nil && spare.alive() {
		return spare, nil
	}
	if spare != nil {
		spare.close()
	}
	return p.manager.providerSnapshot().startAgyProcess()
}

func (p *agyPool) Prewarm() {
	p.mutex.Lock()
	if p.refilling || (p.spare != nil && p.spare.alive()) {
		p.mutex.Unlock()
		return
	}
	p.refilling = true
	generation := p.generation
	p.mutex.Unlock()

	process := p.primedProcess(generation)

	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.refilling = false
	if process == nil {
		return
	}
	if generation != p.generation || p.spare != nil {
		process.close()
		return
	}
	p.spare = process
}

func (p *agyPool) primedProcess(generation int) *agyProcess {
	provider := p.manager.providerSnapshot()
	if selectedEngine(provider.config) != "agy" {
		return nil
	}
	process, err := provider.startAgyProcess()
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), agyBackgroundTimeout)
	defer cancel()
	if _, err := process.ask(ctx, SystemPrompt+"\n\n"+agyPrimePrompt); err != nil {
		process.close()
		return nil
	}
	return process
}

func (p *agyPool) reset() {
	p.mutex.Lock()
	p.generation++
	spare := p.spare
	p.spare = nil
	conversations := p.conversations
	p.conversations = make(map[string]*agyConversation)
	p.mutex.Unlock()

	if spare != nil {
		spare.close()
	}
	for _, conversation := range conversations {
		p.spawn(func() {
			conversation.mutex.Lock()
			defer conversation.mutex.Unlock()
			conversation.drop()
		})
	}
}

func (p *agyPool) spawn(task func()) {
	p.background.Add(1)
	go func() {
		defer p.background.Done()
		task()
	}()
}

func (p *agyPool) wait() {
	p.background.Wait()
}
