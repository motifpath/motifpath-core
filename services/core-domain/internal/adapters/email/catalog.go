package email

import (
	"fmt"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
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
	reminderSubject     map[domain.ReminderStage]string
	reminderBody        map[domain.ReminderStage]func(days int) string
	reminderAction      string
	reminderFooter      map[domain.ReminderStage]string
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

	reminderSubject: map[domain.ReminderStage]string{
		domain.ReminderStageDaily:           "Time for a little practice?",
		domain.ReminderStageInactivity:      "A few minutes of practice today?",
		domain.ReminderStageInactivityAgain: "Pick up where you left off",
	},
	reminderBody: map[domain.ReminderStage]func(days int) string{
		domain.ReminderStageDaily: func(int) string {
			return "You haven't practised yet today. Even five minutes keeps your hands and ears in shape."
		},
		domain.ReminderStageInactivity: func(days int) string {
			return fmt.Sprintf("It's been %d days since your last visit. A short session is enough to keep what you've learned fresh.", days)
		},
		domain.ReminderStageInactivityAgain: func(days int) string {
			return fmt.Sprintf("It's been %d days. Your path is right where you left it, whenever you're ready.", days)
		},
	},
	reminderAction: "Practise now",
	reminderFooter: map[domain.ReminderStage]string{
		domain.ReminderStageDaily:           "You get this email in the evening on days you haven't practised yet.",
		domain.ReminderStageInactivity:      "You get this email after a few days away from MotifPath.",
		domain.ReminderStageInactivityAgain: "You get this email after a few days away from MotifPath.",
	},
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

	reminderSubject: map[domain.ReminderStage]string{
		domain.ReminderStageDaily:           "Hora de praticar um pouco?",
		domain.ReminderStageInactivity:      "Que tal alguns minutos de prática hoje?",
		domain.ReminderStageInactivityAgain: "Retome de onde parou",
	},
	reminderBody: map[domain.ReminderStage]func(days int) string{
		domain.ReminderStageDaily: func(int) string {
			return "Você ainda não praticou hoje. Cinco minutos já bastam para manter as mãos e o ouvido em forma."
		},
		domain.ReminderStageInactivity: func(days int) string {
			return fmt.Sprintf("Já faz %d dias desde a sua última visita. Uma sessão curta já basta para manter vivo o que você aprendeu.", days)
		},
		domain.ReminderStageInactivityAgain: func(days int) string {
			return fmt.Sprintf("Já faz %d dias. Seu caminho está exatamente onde você parou, quando quiser voltar.", days)
		},
	},
	reminderAction: "Praticar agora",
	reminderFooter: map[domain.ReminderStage]string{
		domain.ReminderStageDaily:           "Você recebe este email à noite, nos dias em que ainda não praticou.",
		domain.ReminderStageInactivity:      "Você recebe este email depois de alguns dias longe do MotifPath.",
		domain.ReminderStageInactivityAgain: "Você recebe este email depois de alguns dias longe do MotifPath.",
	},
	reminderUnsubscribe: "Parar lembretes",
}
