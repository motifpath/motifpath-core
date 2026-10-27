package email

import (
	"fmt"
	"time"
)

// catalog is the wording of the student emails in one language. Its
// functions take the figures they show, so each language decides its own
// plurals and date formats. The tone follows the app's: practice first, and
// never a word about a streak being lost.
type catalog struct {
	date     func(time.Time) string
	greeting func(firstName string) string

	reportSubject       string
	reportIntro         func(from, to string) string
	practiceDaysLabel   string
	practiceDays        func(days int) string
	minutesLabel        string
	minutes             func(minutes, previous int) string
	skillsUpLabel       string
	songsPlayedLabel    string
	streak              func(current, best int) string
	newStreak           func(best int) string
	firstStreak         string
	reportAction        string
	reportFooter        string
	reportUnsubscribe   string
	reminderSubject     func(second bool) string
	reminderBody        func(days int, second bool) string
	reminderAction      string
	reminderFooter      string
	reminderUnsubscribe string
}

// catalogs holds every language the emails are written in. A locale with no
// catalog gets English.
var catalogs = map[string]catalog{
	"en":    english,
	"pt_BR": portugueseBrazil,
}

func catalogFor(locale string) catalog {
	if c, ok := catalogs[locale]; ok {
		return c
	}
	return english
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

var english = catalog{
	date: func(t time.Time) string { return t.Format("Jan 2") },
	greeting: func(firstName string) string {
		if firstName == "" {
			return "Hi,"
		}
		return fmt.Sprintf("Hi %s,", firstName)
	},

	reportSubject: "Your week on MotifPath",
	reportIntro: func(from, to string) string {
		return fmt.Sprintf("Here's your week of practice, from %s to %s.", from, to)
	},
	practiceDaysLabel: "Practice days",
	practiceDays:      func(days int) string { return fmt.Sprintf("%d of 7", days) },
	minutesLabel:      "Minutes practised",
	minutes: func(minutes, previous int) string {
		return fmt.Sprintf("%d min (the week before: %d)", minutes, previous)
	},
	skillsUpLabel:    "Skills up",
	songsPlayedLabel: "Songs played",
	streak: func(current, best int) string {
		return fmt.Sprintf("Day streak: %d %s (best: %d)", current, plural(current, "day", "days"), best)
	},
	newStreak: func(best int) string {
		return fmt.Sprintf("Practise today to start a new day streak. Your best is %d %s.", best, plural(best, "day", "days"))
	},
	firstStreak:       "Practise today to start a day streak.",
	reportAction:      "Keep practising",
	reportFooter:      "You get this email on Mondays, after a week in which you practised.",
	reportUnsubscribe: "Stop progress reports",

	reminderSubject: func(second bool) string {
		if second {
			return "Pick up where you left off"
		}
		return "A few minutes of practice today?"
	},
	reminderBody: func(days int, second bool) string {
		if second {
			return fmt.Sprintf("It's been %d days. Your path is right where you left it, whenever you're ready.", days)
		}
		return fmt.Sprintf("It's been %d days since your last visit. A short session is enough to keep what you've learned fresh.", days)
	},
	reminderAction:      "Practise now",
	reminderFooter:      "You get this email after a few days away from MotifPath.",
	reminderUnsubscribe: "Stop reminders",
}

var portugueseBrazil = catalog{
	date: func(t time.Time) string { return t.Format("2/1") },
	greeting: func(firstName string) string {
		if firstName == "" {
			return "Oi,"
		}
		return fmt.Sprintf("Oi, %s,", firstName)
	},

	reportSubject: "Sua semana no MotifPath",
	reportIntro: func(from, to string) string {
		return fmt.Sprintf("Este é o resumo da sua semana de prática, de %s a %s.", from, to)
	},
	practiceDaysLabel: "Dias de prática",
	practiceDays:      func(days int) string { return fmt.Sprintf("%d de 7", days) },
	minutesLabel:      "Minutos praticados",
	minutes: func(minutes, previous int) string {
		return fmt.Sprintf("%d min (semana anterior: %d)", minutes, previous)
	},
	skillsUpLabel:    "Habilidades que avançaram",
	songsPlayedLabel: "Músicas tocadas",
	streak: func(current, best int) string {
		return fmt.Sprintf("Sequência de dias: %d (recorde: %d)", current, best)
	},
	newStreak: func(best int) string {
		return fmt.Sprintf("Pratique hoje para começar uma nova sequência de dias. Seu recorde é de %d %s.", best, plural(best, "dia", "dias"))
	},
	firstStreak:       "Pratique hoje para começar uma sequência de dias.",
	reportAction:      "Continuar praticando",
	reportFooter:      "Você recebe este email às segundas-feiras, depois de uma semana em que praticou.",
	reportUnsubscribe: "Parar os resumos semanais",

	reminderSubject: func(second bool) string {
		if second {
			return "Retome de onde parou"
		}
		return "Que tal alguns minutos de prática hoje?"
	},
	reminderBody: func(days int, second bool) string {
		if second {
			return fmt.Sprintf("Já faz %d dias. Seu caminho está exatamente onde você parou, quando quiser voltar.", days)
		}
		return fmt.Sprintf("Já faz %d dias desde a sua última visita. Uma sessão curta já basta para manter vivo o que você aprendeu.", days)
	},
	reminderAction:      "Praticar agora",
	reminderFooter:      "Você recebe este email depois de alguns dias longe do MotifPath.",
	reminderUnsubscribe: "Parar lembretes",
}
