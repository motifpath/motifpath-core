package main

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// ptBRPhrases is the Portuguese of every English phrase a seeded exercise
// uses: titles, prompt texts and option labels. Note names stay letters, as
// everywhere else in pt-BR.
var ptBRPhrases = map[string]string{
	// One exercise of every exercise_type.
	"Name the interval — text response":  "Nomeie o intervalo — resposta em texto",
	"Perfect fifth":                      "Quinta justa",
	"Major third":                        "Terça maior",
	"Identify the recorded interval":     "Identifique o intervalo gravado",
	"Minor third":                        "Terça menor",
	"Major sixth":                        "Sexta maior",
	"Tap the root note on the fretboard": "Toque a fundamental no braço",
	"Pick the C major chord shape":       "Escolha o desenho do acorde de C maior",
	"Pick the matching recorded lick":    "Escolha a gravação que corresponde ao lick",

	// Hearing chord quality.
	"Major or minor?":               "Maior ou menor?",
	"Is this chord major or minor?": "Este acorde é maior ou menor?",
	"Major or minor, note by note?": "Maior ou menor, nota por nota?",
	"Major":                         "Maior",
	"Minor":                         "Menor",
	"Which one is minor?":           "Qual deles é menor?",
	"Which of these sounds is a minor chord?":                   "Qual destes sons é um acorde menor?",
	"Is this chord, played one note at a time, major or minor?": "Este acorde, tocado uma nota de cada vez, é maior ou menor?",
	"Sound 1":      "Som 1",
	"Sound 2":      "Som 2",
	"Sound 3":      "Som 3",
	listeningTitle: "Ouça o acorde de E maior e toque sua terça maior",

	// Answered on a diagram.
	"Tap every root of the A minor pentatonic":                   "Toque todas as tônicas da pentatônica de A menor",
	"Which diagram shows the E major chord?":                     "Qual diagrama mostra o acorde de E maior?",
	"Tap every A of the A minor pentatonic, up to the 12th fret": "Toque todas as notas A da pentatônica de A menor, até a 12ª casa",
	"Where do pentatonic boxes 1 and 2 meet?":                    "Onde os desenhos 1 e 2 da pentatônica se encontram?",
	"Boxes 1 and 2 of the A minor pentatonic, on one board.":     "Os desenhos 1 e 2 da pentatônica de A menor, num só braço.",
	"They share the notes around the 7th and 8th frets":          "Compartilham as notas em torno das casas 7 e 8",
	"They share no notes":                                        "Não compartilham nenhuma nota",
	"They share only the open strings":                           "Compartilham apenas as cordas soltas",
	"What chord is this?":                                        "Que acorde é este?",
	"A major":                                                    "A maior",
	"A minor":                                                    "A menor",
	"D major":                                                    "D maior",

	// The lessons' challenges.
	"Which three open chords play most songs in G?": "Quais três acordes abertos tocam a maioria das músicas em G?",
	"G, C and D": "G, C e D",
	"A, B and E": "A, B e E",
	"Where should your fingers sit on a fret?":                    "Onde seus dedos devem ficar na casa?",
	"Close to the fret wire":                                      "Perto do traste",
	"In the middle of the fret":                                   "No meio da casa",
	"When should your hand start moving to the next chord?":       "Quando sua mão deve começar a ir para o próximo acorde?",
	"Just before the beat":                                        "Logo antes do tempo",
	"On the beat":                                                 "No tempo",
	"How should your fingers land on the new chord?":              "Como seus dedos devem chegar ao novo acorde?",
	"Together, as one shape":                                      "Juntos, como um só desenho",
	"One at a time":                                               "Um de cada vez",
	"Which note is the root of a C major scale in open position?": "Qual nota é a tônica da escala de C maior na posição aberta?",
	"How many notes does a major scale have before it repeats an octave higher?": "Quantas notas tem uma escala maior antes de se repetir uma oitava acima?",
	"C":     "C",
	"G":     "G",
	"Seven": "Sete",
	"Five":  "Cinco",
}

// translatedExercise is a copy of en with its words in Portuguese from
// phrases, available in pt_BR: its title, every text of its prompt and every
// option label, its options under new ids. One whose answers are tapped on a
// diagram leaves its options out, to be derived from the diagram again. A
// phrase phrases lacks is refused.
func translatedExercise(en domain.Exercise, phrases map[string]string) (domain.Exercise, error) {
	translate := func(s string) (string, error) {
		if pt, ok := phrases[s]; ok {
			return pt, nil
		}
		return "", fmt.Errorf("no Portuguese for %q", s)
	}
	pt := en
	var err error
	if pt.Title, err = translate(en.Title); err != nil {
		return domain.Exercise{}, err
	}
	if pt.Prompt.Content, err = translatedNodes(en.Prompt.Content, translate); err != nil {
		return domain.Exercise{}, err
	}
	pt.Options = nil
	tappedOnDiagram := en.ExerciseType == domain.ExerciseTypeImageRecognition && (en.DiagramRef != nil || en.DiagramStackRef != nil)
	if !tappedOnDiagram {
		if pt.Options, err = translatedOptions(en.Options, translate); err != nil {
			return domain.Exercise{}, err
		}
	}
	pt.Languages = []domain.Language{{Code: "pt_BR"}}
	return pt, nil
}

// translatedOptions is a copy of options under new ids, every label
// translated.
func translatedOptions(options []domain.Option, translate func(string) (string, error)) ([]domain.Option, error) {
	var out []domain.Option
	for _, o := range options {
		o.ID = uuid.NewString()
		if o.Label != nil {
			label, err := translate(*o.Label)
			if err != nil {
				return nil, err
			}
			o.Label = &label
		}
		out = append(out, o)
	}
	return out, nil
}

// translatedNodes is a copy of nodes with every text translated.
func translatedNodes(nodes []domain.PromptNode, translate func(string) (string, error)) ([]domain.PromptNode, error) {
	if nodes == nil {
		return nil, nil
	}
	out := make([]domain.PromptNode, len(nodes))
	for i, n := range nodes {
		var err error
		if n.Text != "" {
			if n.Text, err = translate(n.Text); err != nil {
				return nil, err
			}
		}
		if n.Content, err = translatedNodes(n.Content, translate); err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

// seedPortugueseExercises gives every English exercise teacher seeded a
// Portuguese copy, so a pt-BR student has the same exercises to practise. A
// copy is classified, illustrated and sized like its original but linked to
// no challenge, since the challenges are on English lessons.
func seedPortugueseExercises(ctx context.Context, teacher domain.User, exerciseSvc *application.ExerciseService) error {
	var english []domain.Exercise
	page := domain.PageRequest{Limit: domain.MaxPageLimit}
	for {
		got, err := exerciseSvc.ListExercises(ctx, teacher, domain.ExerciseFilter{Language: "en", CreatedBy: teacher.ID}, page)
		if err != nil {
			return fmt.Errorf("list the seeded English exercises: %w", err)
		}
		english = append(english, got.Items...)
		page.Offset += len(got.Items)
		if len(got.Items) == 0 || page.Offset >= got.Total {
			break
		}
	}
	for _, en := range english {
		pt, err := translatedExercise(en, ptBRPhrases)
		if err != nil {
			return fmt.Errorf("translate exercise %q: %w", en.Title, err)
		}
		instrumentIDs := slices.Clone(pt.InstrumentIDs)
		if _, err := exerciseSvc.CreateExercise(ctx, teacher, pt.Title, pt.Prompt, pt.ExerciseType, nodeIDs(pt.Skills), nodeIDs(pt.Concepts),
			pt.ImageURL, pt.AudioURL, pt.DiagramRef, pt.DiagramStackRef, pt.Options, pt.EstimatedDurationSeconds, nil, []string{"pt_BR"}, &instrumentIDs); err != nil {
			return fmt.Errorf("create the Portuguese copy of %q: %w", en.Title, err)
		}
	}
	log.Printf("seeded a Portuguese copy of each of the %d English exercises, for pt-BR students to practise (linked to no challenge)", len(english))
	return nil
}

// nodeIDs lists the ids of nodes.
func nodeIDs(nodes []domain.KnowledgeNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}
