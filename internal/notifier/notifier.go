package notifier

import (
	//_ "net/http/pprof"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	m "validators-health/internal/models"
	"validators-health/internal/services"
)

var ctx = context.Background()

const GlobalSubscriptionKey = "global_subscribers"
const MaxMessagesPerMinute = 20

var adminChatIDs = []int64{
	1531459, // @mobyman
}

func NewNotifier(clickhouseService *services.ClickhouseService, cacheService *services.CacheService) (*Notifier, error) {
	apiToken := os.Getenv("TELEGRAM_API_KEY")
	n := &Notifier{}
	botClient, err := bot.New(apiToken, bot.WithDefaultHandler(n.defaultHandler))
	n.bot = botClient
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %v", err)
	}
	n.redisClient = cacheService.RedisClient
	n.ClickhouseService = clickhouseService

	return n, nil
}

type Notifier struct {
	bot               *bot.Bot
	redisClient       *redis.Client
	ClickhouseService *services.ClickhouseService
}

type Alert struct {
	ID                  int64             `json:"id"`
	ADNLAddr            string            `json:"adnl_addr"`
	ValidatorADNL       string            `json:"validator_adnl"`
	Status              m.ValidatorStatus `json:"status"`
	IsAcknowledged      bool              `json:"is_acknowledged"`
	AckBy               int64             `json:"ack_by,omitempty"`
	AckByUsername       string            `json:"ack_by_username,omitempty"`
	LastAlert           time.Time         `json:"last_alert"`
	Efficiency          float64           `json:"efficiency"`
	PreviousStatus      string            `json:"previous_status,omitempty"`
	PreviousStatusSince time.Time         `json:"previous_status_since,omitempty"`
	Duration            time.Duration     `json:"duration,omitempty"`
	Timestamp           uint32            `json:"timestamp,omitempty"`
}

type Subscription struct {
	ChatID    int64  `json:"chat_id"`
	Timestamp int64  `json:"timestamp"`
	ADNL      string `json:"adnl"`
}

func (n *Notifier) ListenAndNotify(stop <-chan struct{}) {
	go n.HandleUpdates()
	subscriber := n.redisClient.Subscribe(ctx, "validator_notifications")
	msgs := subscriber.Channel()

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				log.Println("Subscription channel closed")
				return
			}

			var alert Alert
			err := json.Unmarshal([]byte(msg.Payload), &alert)
			if err != nil {
				log.Printf("Failed to unmarshal alert: %v", err)
				continue
			}

			subscriptionKey := fmt.Sprintf("subscription_%s", alert.ValidatorADNL)
			subscriptions, err := n.redisClient.SMembers(ctx, subscriptionKey).Result()
			if err != nil {
				log.Printf("Failed to get subscriptions for ADNLAddr %s: %v", alert.ValidatorADNL, err)
				continue
			}

			message := n.formatAlertMessage(alert)
			if len(subscriptions) == 0 {
				defaultUsers := []int64{} // add default users for all notifications
				for _, chatID := range defaultUsers {
					n.sendMessage(chatID, message, alert)
				}
			} else {

				for _, chatIDStr := range subscriptions {
					chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
					if err != nil {
						log.Printf("Invalid chat ID: %v", err)
						continue
					}
					n.sendMessage(chatID, message, alert)
				}
			}
		case <-stop:
			log.Println("Notifier is shutting down.")
			return
		case <-time.After(5 * time.Second):

			log.Println("No messages in subscription channel, waiting...")
		}
	}
}

func (n *Notifier) formatAlertMessage(alert Alert) string {
	statusEmoji := "✅"
	if alert.Status == m.StatusNotOK {
		statusEmoji = "❌"
	}

	message := fmt.Sprintf("%s %s\nValidator %s is now %s", statusEmoji, time.Now().Format("2006-01-02 15:04:05"), alert.ValidatorADNL, alert.Status)
	if alert.PreviousStatus != "" && alert.PreviousStatus != "unknown" {
		duration := alert.Duration
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		message += fmt.Sprintf("\nPrevious state %s, duration: %dh %d min.", alert.PreviousStatus, hours, minutes)
	}
	message += fmt.Sprintf("\n\nCheck details at: https://%s/?adnl=%s&from=%d&to=%d", os.Getenv("HOSTNAME"), alert.ValidatorADNL, alert.Timestamp, alert.Timestamp+uint32(time.Hour.Seconds()))

	return message
}

func (n *Notifier) sendMessage(chatID int64, message string, alert Alert) {
	rateLimitKey := fmt.Sprintf("rate_limit_%d_%d", chatID, time.Now().Unix())
	messageCount, err := n.redisClient.Incr(ctx, rateLimitKey).Result()
	if err != nil {
		log.Printf("Failed to increment rate limit key for chatID %d: %v", chatID, err)
		return
	}
	if messageCount == 1 {
		err := n.redisClient.Expire(ctx, rateLimitKey, time.Minute).Err()
		if err != nil {
			log.Printf("Failed to set expiration for rate limit key for chatID %d: %v", chatID, err)
		}
	}

	if messageCount > MaxMessagesPerMinute {
		log.Printf("Rate limit hit for chatID %d, skipping message", chatID)
		return
	}

	msg := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,
	}

	if alert.Status == m.StatusNotOK {
		ackButton := &models.InlineKeyboardButton{
			Text:         "Acknowledge",
			CallbackData: "ack_" + strconv.FormatInt(alert.ID, 10),
		}
		msg.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{*ackButton},
			},
		}
	}

	_, err = n.bot.SendMessage(ctx, msg)
	if err != nil {
		log.Printf("Failed to send message to chat %d: %v", chatID, err)
	}
}

func (n *Notifier) HandleUpdates() {
	n.bot.RegisterHandler(bot.HandlerTypeMessageText, "/add", bot.MatchTypePrefix, n.handleAdd)
	n.bot.RegisterHandler(bot.HandlerTypeMessageText, "/del", bot.MatchTypePrefix, n.handleDel)
	n.bot.RegisterHandler(bot.HandlerTypeMessageText, "/announce", bot.MatchTypePrefix, n.handleAnnounce)
	n.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "", bot.MatchTypePrefix, n.handleCallback)

	n.bot.Start(ctx)
}

func (n *Notifier) handleCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	callback := update.CallbackQuery
	data := callback.Data
	if len(data) > 4 && data[:4] == "ack_" {
		alertId := data[4:]
		alertKey := "alert_" + alertId
		alertData, err := n.redisClient.Get(ctx, alertKey).Result()
		if errors.Is(err, redis.Nil) {
			if callback.Message.Message != nil {
				chatID := callback.Message.Message.Chat.ID
				msg := &bot.SendMessageParams{
					ChatID: chatID,
					Text:   "No such alert.",
				}
				n.bot.SendMessage(ctx, msg)
			}
			return
		}

		var alert Alert
		json.Unmarshal([]byte(alertData), &alert)
		alert.IsAcknowledged = true
		alert.AckBy = callback.From.ID
		n.redisClient.Set(ctx, alertKey, alertData, 0)
		var currentStatus m.ValidatorStatus
		currentStatus = m.StatusAcknowledged
		err = n.ClickhouseService.InsertStatusChange(alert.ADNLAddr, alert.ValidatorADNL, currentStatus, time.Now())
		if err != nil {
			log.Printf("Failed to insert status change into ClickHouse: %v", err)
		}

		if callback.Message.Message != nil {
			chatID := callback.Message.Message.Chat.ID
			var userName string
			if callback.From.Username != "" {
				userName = callback.From.Username
			} else {
				userName = "user"
			}

			msg := &bot.SendMessageParams{
				ChatID:    chatID,
				Text:      fmt.Sprintf("[%s] 🚑 Acknowledged by [%s](tg://user?id\\=%d)", time.Now().Format("2006\\-01\\-02 15:04:05"), userName, callback.From.ID),
				ParseMode: models.ParseModeMarkdown,
			}
			_, err := n.bot.SendMessage(ctx, msg)
			if err != nil {
				log.Printf("Failed to send message to chat %d: %v", chatID, err)
				return
			}
		}
	}
}

func (n *Notifier) handleAdd(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	chatID := update.Message.Chat.ID
	args := strings.Split(update.Message.Text, " ")
	if len(args) < 2 {
		msg := &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Usage: /add <ADNL>",
		}
		n.bot.SendMessage(ctx, msg)
		return
	}

	adnl := args[1]
	adnlPattern := `^[A-F0-9]{64}$`
	matched, err := regexp.MatchString(adnlPattern, adnl)
	if err != nil || !matched {
		msg := &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Invalid ADNL format. ADNL must be a 64-character hex string (uppercase, A-F, 0-9).",
		}
		_, err := n.bot.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("Failed to send message to chat %d: %v", chatID, err)
			return
		}
		return
	}

	subscriptionKey := fmt.Sprintf("subscription_%s", adnl)

	err = n.redisClient.SAdd(ctx, subscriptionKey, chatID).Err()
	if err != nil {
		log.Printf("Failed to add subscription for ADNL %s: %v", adnl, err)
		msg := &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Failed to subscribe to alerts.",
		}
		_, err := n.bot.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("Failed to send message to chat %d: %v", chatID, err)
			return
		}
		return
	}

	err = n.redisClient.SAdd(ctx, GlobalSubscriptionKey, chatID).Err()
	if err != nil {
		log.Printf("Failed to add to global subscribers list: %v", err)
	}

	msg := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("Subscribed to alerts for ADNL: %s", adnl),
	}
	_, err = n.bot.SendMessage(ctx, msg)
	if err != nil {
		log.Printf("Failed to send message to chat %d: %v", chatID, err)
		return
	}
}

func (n *Notifier) handleDel(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	chatID := update.Message.Chat.ID
	args := strings.Split(update.Message.Text, " ")
	if len(args) < 2 {
		msg := &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Usage: /del <ADNL>",
		}
		_, err := n.bot.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("Failed to send message to chat %d: %v", chatID, err)
			return
		}
		return
	}

	adnl := args[1]
	subscriptionKey := fmt.Sprintf("subscription_%s", adnl)

	err := n.redisClient.SRem(ctx, subscriptionKey, chatID).Err()
	if err != nil {
		log.Printf("Failed to remove subscription for ADNL %s: %v", adnl, err)
		msg := &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Failed to unsubscribe from alerts.",
		}
		_, err := n.bot.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("Failed to send message to chat %d: %v", chatID, err)
			return
		}
		return
	}

	subscriptions, err := n.redisClient.Keys(ctx, "subscription_*").Result()
	if err != nil {
		log.Printf("Failed to check subscriptions for chat %d: %v", chatID, err)
		return
	}

	isSubscribedToOtherADNL := false
	for _, subKey := range subscriptions {
		isMember, err := n.redisClient.SIsMember(ctx, subKey, chatID).Result()
		if err == nil && isMember {
			isSubscribedToOtherADNL = true
			break
		}
	}

	if !isSubscribedToOtherADNL {
		err := n.redisClient.SRem(ctx, GlobalSubscriptionKey, chatID).Err()
		if err != nil {
			log.Printf("Failed to remove from global subscribers list: %v", err)
		}
	}

	msg := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("Unsubscribed from alerts for ADNL: %s", adnl),
	}
	_, err = n.bot.SendMessage(ctx, msg)
	if err != nil {
		log.Printf("Failed to send message to chat %d: %v", chatID, err)
		return
	}
}

func (n *Notifier) handleAnnounce(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	if !n.isAdmin(update.Message.Chat.ID) {
		msg := &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "You are not authorized to use this command.",
		}
		_, err := n.bot.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("Failed to send message to chat %d: %v", update.Message.Chat.ID, err)
			return
		}
		return
	}

	args := strings.SplitN(update.Message.Text, " ", 2)
	if len(args) < 2 {
		msg := &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Usage: /announce <message>",
		}
		_, err := n.bot.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("Failed to send message to chat %d: %v", update.Message.Chat.ID, err)
			return
		}
		return
	}

	announcement := args[1]

	subscribers, err := n.redisClient.SMembers(ctx, GlobalSubscriptionKey).Result()
	if err != nil {
		log.Printf("Failed to get global subscribers: %v", err)
		msg := &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to send announcement.",
		}
		_, err := n.bot.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("Failed to send message to chat %d: %v", update.Message.Chat.ID, err)
			return
		}
		return
	}

	for _, chatIDStr := range subscribers {
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			log.Printf("Invalid chat ID: %v", err)
			continue
		}

		msg := &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("📢 Announcement:\n\n%s", announcement),
			ParseMode: "Markdown",
		}

		_, err = n.bot.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("Failed to send message to chat %d: %v", chatID, err)
			return
		}
	}

	log.Printf("Announcement sent to %d subscribers.", len(subscribers))
}

func (n *Notifier) isAdmin(chatID int64) bool {
	for _, adminID := range adminChatIDs {
		if chatID == adminID {
			return true
		}
	}
	return false
}

func (n *Notifier) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil && strings.HasPrefix(update.Message.Text, "/start") {
		args := strings.Split(update.Message.Text, " ")
		if len(args) > 1 && strings.HasPrefix(args[1], "add_") {
			adnl := strings.TrimPrefix(args[1], "add_")
			update.Message.Text = "/add " + adnl
			n.handleAdd(ctx, b, update)
		}
	}

	if update.Message != nil {
		msg := &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Unknown command. Available commands:\n/add <ADNL> - Subscribe to alerts\n/del <ADNL> - Unsubscribe from alerts",
		}
		_, err := b.SendMessage(ctx, msg)
		if err != nil {
			return
		}
	}
}

func (n *Notifier) PublishAlert(alert Alert) error {
	alertJSON, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to serialize alert: %w", err)
	}

	alertKey := fmt.Sprintf("alert_%d", alert.ID)
	err = n.redisClient.Set(ctx, alertKey, alertJSON, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to save alert to Redis: %w", err)
	}

	err = n.redisClient.Publish(ctx, "validator_notifications", alertJSON).Err()
	if err != nil {
		return fmt.Errorf("failed to publish alert to Redis: %w", err)
	}

	return nil
}
