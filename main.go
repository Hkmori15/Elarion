package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/translate"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/text/language"
	"google.golang.org/api/option"
	tele "gopkg.in/telebot.v4"

	"github.com/Hkmori15/Elarion/db"
)

var (
	// Debug
	errorLog = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	infoLog  = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)

	// Language buttons
	menu = &tele.ReplyMarkup{
		ResizeKeyboard: true,
	}

	btnRuEn  = menu.Text("🇷🇺 RU → 🇬🇧 EN")
	btnEnRu  = menu.Text("🇬🇧 EN → 🇷🇺 RU")
	btnUkrEn = menu.Text("🇺🇦 UA → 🇬🇧 EN")
	btnEnUkr = menu.Text("🇬🇧 EN → 🇺🇦 UA")
	btnEnJp  = menu.Text("🇬🇧 EN → 🇯🇵 JP")
	btnJpEn  = menu.Text("🇯🇵 JP → 🇬🇧 EN")
)

// Track user's selection trans direction
var userStates = make(map[int64]string)

func logError(err error, userId int64, command string) {
	errorLog.Printf("UserID: %d, Command: %s, Error: %v", userId, command, err)
}

func logInfo(message string, userId int64) {
	infoLog.Printf("UserID: %d, Message: %s", userId, message)
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen] + "..."
}

func main() {
	godotenv.Load()

	pref := tele.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	database := db.InitDB()
	db.InitStatsTable(database)

	if err != nil {
		log.Fatal(err)
		return
	}

	log.Printf("Bot connected successfully")

	ctx := context.Background()
	client, err := translate.NewClient(ctx, option.WithAPIKey(os.Getenv("GOOGLE_API_KEY")))

	if err != nil {
		log.Fatal(err)
	}

	defer client.Close()

	b.Handle("/start", func(c tele.Context) error {
		menu.Reply(
			menu.Row(btnRuEn, btnEnRu),
			menu.Row(btnUkrEn, btnEnUkr),
			menu.Row(btnEnJp, btnJpEn),
		)

		return c.Send("Добро пожаловать в Elarion! Бота для перевода текста на любой язык. Для перевода используйте кнопки, либо же формат использования:\n/translate [target lang code] [text] \nПример: /translate ru Hello World. Для просмотра историй переводов используй команду /history. Для просмотра статистики /stats. Чтобы просмотреть доступные языки и их коды, напиши: /languages.", menu)
	})

	b.Handle(&btnRuEn, func(c tele.Context) error {
		userStates[c.Sender().ID] = "ru-en"
		return c.Send("Введите текст для перевода с ru на en:")
	})

	b.Handle(&btnEnRu, func(c tele.Context) error {
		userStates[c.Sender().ID] = "en-ru"
		return c.Send("Enter text to translate from en to ru:")
	})

	b.Handle(&btnUkrEn, func(c tele.Context) error {
		userStates[c.Sender().ID] = "ukr-en"
		return c.Send("Введіть текст для перекладу з ukr на en:")
	})

	b.Handle(&btnEnUkr, func(c tele.Context) error {
		userStates[c.Sender().ID] = "en-ukr"
		return c.Send("Enter text to translate from en to ukr:")
	})

	b.Handle(&btnEnJp, func(c tele.Context) error {
		userStates[c.Sender().ID] = "en-ja"
		return c.Send("Enter text to translate from en to ja:")
	})

	b.Handle(&btnJpEn, func(c tele.Context) error {
		userStates[c.Sender().ID] = "ja-en"
		return c.Send("ja から en に翻訳するテキストを入力してください:")
	})

	b.Handle(tele.OnText, func(c tele.Context) error {
		state, exists := userStates[c.Sender().ID]

		if !exists {
			return nil // ignore if no translation was req
		}

		text := c.Text()

		var targetLang language.Tag

		switch state {
		case "ru-en":
			targetLang = language.English

		case "en-ru":
			targetLang = language.Russian

		case "ukr-en":
			targetLang = language.English

		case "en-ukr":
			targetLang = language.Ukrainian

		case "en-ja":
			targetLang = language.Japanese

		case "ja-en":
			targetLang = language.English

		default:
			return nil
		}

		translation, err := client.Translate(
			ctx,
			[]string{text},
			targetLang,
			&translate.Options{Format: translate.Text},
		)

		if err != nil {
			return c.Send("Не удалось перевести: " + err.Error())
		}

		_, err = db.InitDB().Exec(
			"INSERT INTO translations (user_id, username, original, translated, from_lang, to_lang) VALUES (?, ?, ?, ?, ?, ?)",
			c.Sender().ID,
			c.Sender().Username,
			text,
			translation[0].Text,
			"auto",
			targetLang.String(),
		)

		if err != nil {
			log.Printf("Failed to save translation history: %v", err)
			return c.Send("Перевод был выполнен, но сохранить историю перевода не удалось")
		}

		_, err = db.InitDB().Exec(`
			INSERT INTO usage_stats (user_id, username, translation_count, last_used)
			VALUES (?, ?, 1, CURRENT_TIMESTAMP)
			ON CONFLICT (user_id) DO UPDATE SET
			translation_count = translation_count + 1,
			last_used = CURRENT_TIMESTAMP
		`, c.Sender().ID, c.Sender().Username)

		if err != nil {
			log.Printf("Failed to update usage stats: %v", err)
		}

		// Clear the state after translation
		delete(userStates, c.Sender().ID)

		return c.Send(translation[0].Text)
	})

	b.Handle("/history", func(c tele.Context) error {
		rows, err := db.InitDB().Query("SELECT original, translated, to_lang, created_at FROM translations WHERE user_id = ? ORDER BY created_at DESC LIMIT 5", c.Sender().ID)

		if err != nil {
			return c.Send("Ошибка при получении истории переводов")
		}

		defer rows.Close()

		var history []string
		const maxTextLen = 500 // limit for each text field

		for rows.Next() {
			var t db.Translation

			err := rows.Scan(&t.Original, &t.Translated, &t.ToLang, &t.CreatedAt)

			if err != nil {
				continue
			}

			history = append(history, fmt.Sprintf(
				"🔄 %s\n📝 %s → %s\n⏰ %s\n",
				truncateText(t.Original, maxTextLen),
				truncateText(t.Translated, maxTextLen),
				t.ToLang,
				t.CreatedAt.Format("02.01.2006 15:04"),
			))
		}

		if len(history) == 0 {
			return c.Send("История переводов пуста")
		}

		return c.Send("Ваши последние переводы:\n\n" + strings.Join(history, "\n"))
	})

	b.Handle("/stats", func(c tele.Context) error {
		var stats db.Stats

		err := db.InitDB().QueryRow("SELECT translation_count, last_used FROM usage_stats WHERE user_id = ?", c.Sender().ID).Scan(&stats.TranslationCount, &stats.LastUsed)

		if err != nil {
			return c.Send("У вас еще нет статистики")
		}

		return c.Send(fmt.Sprintf(
			"📊 Ваша статистика:\nПереводов: %d\nПоследнее использование: %s",
			stats.TranslationCount,
			stats.LastUsed.Format("02.01.2006 15:04"),
		))
	})

	b.Handle("/languages", func(c tele.Context) error {
		langs, err := client.SupportedLanguages(ctx, language.Russian)

		if err != nil {
			return c.Send("Ошибка получения списка языков")
		}

		var langList []string

		for _, lang := range langs {
			langList = append(langList, fmt.Sprintf("%v (%v)", lang.Tag, lang.Name))
		}

		// Format the output in col
		res := "Доступные языки:\n\n" + strings.Join(langList, "\n")

		return c.Send(res)
	})

	b.Handle("/translate", func(c tele.Context) error {
		logInfo("Translation req", c.Sender().ID)

		args := c.Args()

		if len(args) < 2 {
			logError(fmt.Errorf("invalid format"), c.Sender().ID, "/translate")
			return c.Send("Используйте формат: /translate [target lang code] [text]")
		}

		targetLang, err := language.Parse(args[0])

		if err != nil {
			logError(err, c.Sender().ID, "/translate")
			return c.Send("Неверный код языка. Используйте ISO 639-1 коды // en, es, ru и.т.д")
		}

		text := []string{strings.Join(args[1:], " ")}

		translation, err := client.Translate(
			ctx,
			text,
			targetLang,
			&translate.Options{
				Format: translate.Text,
			})

		if err != nil {
			logError(err, c.Sender().ID, "/translate")
			return c.Send("Технические шоколадки: " + err.Error())
		}

		if len(translation) == 0 {
			return c.Send("Не удалось перевести текст")
		}

		_, err = db.InitDB().Exec(
			"INSERT INTO translations (user_id, username, original, translated, from_lang, to_lang) VALUES (?, ?, ?, ?, ?, ?)",
			c.Sender().ID,
			c.Sender().Username,
			strings.Join(args[1:], " "),
			translation[0].Text,
			"auto",
			targetLang.String(),
		)

		if err != nil {
			log.Printf("Failed to save translation history: %v", err)
			return c.Send("Перевод был выполнен, но сохранить историю перевода не удалось")
		}

		_, err = db.InitDB().Exec(`
			INSERT INTO usage_stats (user_id, username, translation_count, last_used)
			VALUES (?, ?, 1, CURRENT_TIMESTAMP)
			ON CONFLICT (user_id) DO UPDATE SET
			translation_count = translation_count + 1,
			last_used = CURRENT_TIMESTAMP
		`, c.Sender().ID, c.Sender().Username)

		if err != nil {
			log.Printf("Failed to update usage stats: %v", err)
		}

		logInfo("Translation successful", c.Sender().ID)
		return c.Send(translation[0].Text)
	})

	b.Start()
}
