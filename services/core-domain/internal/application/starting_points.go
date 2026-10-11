package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// StartingPointsConfigKey is the setting that holds the team's starting points.
const StartingPointsConfigKey = "starting_points"

// catalogStartingPointsLimit is how many starting points the catalog offers when the team chose
// none: enough to choose from without turning a first visit into browsing.
const catalogStartingPointsLimit = 5

// startingPointsKey checks a starting points value: its shape, then that every instrument exists,
// every language is a real one, and every entry names a published path or course of its kind.
type startingPointsKey struct {
	instruments ports.InstrumentRepository
	languages   ports.LanguageRepository
	paths       ports.LearningPathRepository
	courses     ports.CourseRepository
}

func NewStartingPointsKey(instruments ports.InstrumentRepository, languages ports.LanguageRepository, paths ports.LearningPathRepository, courses ports.CourseRepository) ConfigKey {
	return &startingPointsKey{instruments: instruments, languages: languages, paths: paths, courses: courses}
}

func (k *startingPointsKey) Key() string { return StartingPointsConfigKey }

func (k *startingPointsKey) Normalize(ctx context.Context, raw []byte) ([]byte, error) {
	config, errs := domain.ParseStartingPointsConfig(raw)
	if len(errs) > 0 {
		return nil, &domain.ValidationError{Fields: errs}
	}
	for i, list := range config.Lists {
		at := fmt.Sprintf("lists[%d]", i)
		listErrs, err := k.checkList(ctx, at, list)
		if err != nil {
			return nil, err
		}
		errs = append(errs, listErrs...)
	}
	if len(errs) > 0 {
		return nil, &domain.ValidationError{Fields: errs}
	}
	return json.Marshal(config)
}

func (k *startingPointsKey) checkList(ctx context.Context, at string, list domain.StartingPointsList) ([]domain.FieldError, error) {
	var errs []domain.FieldError
	if _, err := k.instruments.GetByID(ctx, list.InstrumentID); errors.Is(err, domain.ErrNotFound) {
		errs = append(errs, domain.FieldError{Field: at + ".instrument_id", Reason: "names no instrument"})
	} else if err != nil {
		return nil, err
	}
	reason, err := languageProblem(ctx, k.languages, list.Language)
	if err != nil {
		return nil, err
	}
	if reason != "" {
		errs = append(errs, domain.FieldError{Field: at + ".language", Reason: reason})
	}
	for j, item := range list.Items {
		_, published, err := resolveStartingPoint(ctx, k.paths, k.courses, item)
		if err != nil {
			return nil, err
		}
		if !published {
			errs = append(errs, domain.FieldError{Field: fmt.Sprintf("%s.items[%d].id", at, j), Reason: fmt.Sprintf("names no published %s", item.Kind)})
		}
	}
	return errs, nil
}

// languageProblem says why code can't be a starting points language, or "" when it can: it must be
// a known language, and never the language-agnostic marker.
func languageProblem(ctx context.Context, languages ports.LanguageRepository, code string) (string, error) {
	if code == domain.LanguageCodeAny {
		return "must be a language, not the language-agnostic marker", nil
	}
	if _, err := languages.GetByCode(ctx, code); errors.Is(err, domain.ErrNotFound) {
		return "names no language", nil
	} else if err != nil {
		return "", err
	}
	return "", nil
}

// resolveStartingPoint returns the published path or course ref names; ok is false when it names
// nothing, or something not (or no longer) published.
func resolveStartingPoint(ctx context.Context, paths ports.LearningPathRepository, courses ports.CourseRepository, ref domain.StartingPointRef) (StartingPoint, bool, error) {
	switch ref.Kind {
	case domain.StartingPointKindPath:
		path, err := paths.GetByID(ctx, ref.ID)
		if errors.Is(err, domain.ErrNotFound) {
			return StartingPoint{}, false, nil
		}
		if err != nil {
			return StartingPoint{}, false, err
		}
		if path.Status != domain.LearningPathStatusPublished {
			return StartingPoint{}, false, nil
		}
		return StartingPoint{Kind: ref.Kind, Path: &path}, true, nil
	case domain.StartingPointKindCourse:
		course, err := courses.GetByID(ctx, ref.ID)
		if errors.Is(err, domain.ErrNotFound) {
			return StartingPoint{}, false, nil
		}
		if err != nil {
			return StartingPoint{}, false, err
		}
		if course.Status != domain.CourseStatusPublished {
			return StartingPoint{}, false, nil
		}
		return StartingPoint{Kind: ref.Kind, Course: &course}, true, nil
	}
	return StartingPoint{}, false, nil
}

// StartingPointsSource says which answered a starting points question.
type StartingPointsSource string

const (
	// StartingPointsSourceCurated means the team's list answered.
	StartingPointsSourceCurated StartingPointsSource = "curated"
	// StartingPointsSourceCatalog means the team chose nothing still published, so the catalog
	// answered.
	StartingPointsSourceCatalog StartingPointsSource = "catalog"
)

// StartingPoint is a published path or course a new student can start with: Path is set when Kind
// is path, Course when Kind is course.
type StartingPoint struct {
	Kind   domain.StartingPointKind
	Path   *domain.LearningPath
	Course *domain.Course
}

// StartingPoints is where a new student should start, first shown first.
type StartingPoints struct {
	Source StartingPointsSource
	Items  []StartingPoint
}

// StartingPointsService answers where a student who has never started anything should start.
type StartingPointsService struct {
	config    ports.AppConfigRepository
	languages ports.LanguageRepository
	paths     ports.LearningPathRepository
	courses   ports.CourseRepository
}

func NewStartingPointsService(config ports.AppConfigRepository, languages ports.LanguageRepository, paths ports.LearningPathRepository, courses ports.CourseRepository) *StartingPointsService {
	return &StartingPointsService{config: config, languages: languages, paths: paths, courses: courses}
}

// List returns the team's list for instrumentID, experience and language, in its order, without
// anything no longer published. With no such list, or nothing in it still published, it returns
// the catalog's published paths, then courses, for that instrument and language at the
// experience's levels, each by title, at most five in all.
func (s *StartingPointsService) List(ctx context.Context, instrumentID string, experience domain.StartingPointExperience, language string) (StartingPoints, error) {
	if !experience.Valid() {
		return StartingPoints{}, domain.NewValidationError("experience", "must be never_played, plays_a_bit or plays_well")
	}
	reason, err := languageProblem(ctx, s.languages, language)
	if err != nil {
		return StartingPoints{}, err
	}
	if reason != "" {
		return StartingPoints{}, domain.NewValidationError("language", reason)
	}

	curated, err := s.curated(ctx, instrumentID, experience, language)
	if err != nil {
		return StartingPoints{}, err
	}
	if len(curated) > 0 {
		return StartingPoints{Source: StartingPointsSourceCurated, Items: curated}, nil
	}
	fromCatalog, err := s.fromCatalog(ctx, instrumentID, experience, language)
	if err != nil {
		return StartingPoints{}, err
	}
	return StartingPoints{Source: StartingPointsSourceCatalog, Items: fromCatalog}, nil
}

func (s *StartingPointsService) curated(ctx context.Context, instrumentID string, experience domain.StartingPointExperience, language string) ([]StartingPoint, error) {
	entry, err := s.config.Get(ctx, StartingPointsConfigKey)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	config, errs := domain.ParseStartingPointsConfig(entry.Value)
	if len(errs) > 0 {
		return nil, fmt.Errorf("stored starting points no longer parse: %w", &domain.ValidationError{Fields: errs})
	}
	list, ok := config.ListFor(instrumentID, experience, language)
	if !ok {
		return nil, nil
	}
	items := make([]StartingPoint, 0, len(list.Items))
	for _, ref := range list.Items {
		point, ok, err := resolveStartingPoint(ctx, s.paths, s.courses, ref)
		if err != nil {
			return nil, err
		}
		if ok {
			items = append(items, point)
		}
	}
	return items, nil
}

func (s *StartingPointsService) fromCatalog(ctx context.Context, instrumentID string, experience domain.StartingPointExperience, language string) ([]StartingPoint, error) {
	levels := experience.Levels()
	paths, err := s.paths.List(ctx, catalogPathFilter(domain.LearningPathFilter{Levels: levels, InstrumentID: instrumentID, Language: language}), domain.PageRequest{Limit: catalogStartingPointsLimit})
	if err != nil {
		return nil, err
	}
	items := make([]StartingPoint, 0, catalogStartingPointsLimit)
	for _, p := range paths.Items {
		items = append(items, StartingPoint{Kind: domain.StartingPointKindPath, Path: &p})
	}
	remaining := catalogStartingPointsLimit - len(items)
	if remaining == 0 {
		return items, nil
	}
	courses, err := s.courses.List(ctx, catalogCourseFilter(domain.CourseListFilter{Levels: levels, InstrumentID: instrumentID, Language: language}), domain.PageRequest{Limit: remaining})
	if err != nil {
		return nil, err
	}
	for _, c := range courses.Items {
		items = append(items, StartingPoint{Kind: domain.StartingPointKindCourse, Course: &c})
	}
	return items, nil
}
