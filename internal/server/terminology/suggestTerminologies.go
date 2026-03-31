package terminology

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) SuggestTerminologies(
	ctx context.Context,
	req *connect.Request[pb.SuggestTerminologiesRequest],
) (*connect.Response[pb.SuggestTerminologiesResponse], error) {
	if req.Msg.SourceText == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("source text is required"))
	}

	if req.Msg.TargetLocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target locale id is required"))
	}

	// Verify target locale exists
	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.TargetLocaleId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("target locale not found"))
	}

	// Get all terminologies with their definitions
	var terminologies []models.Terminology
	if err := s.db.Preload("Definitions", "locale_id = ?", req.Msg.TargetLocaleId).Find(&terminologies).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Tokenize source text (simple word-based tokenization)
	sourceWords := tokenize(req.Msg.SourceText)

	// Find matching terminologies
	var suggestions []*pb.TerminologySuggestion

	for _, term := range terminologies {
		// Check if terminology term appears in source text (case-insensitive)
		matchedWords := findMatches(term.Term, sourceWords)

		if len(matchedWords) > 0 {
			// Get the definition for the target locale
			var targetTranslation string
			for _, def := range term.Definitions {
				if def.LocaleID == req.Msg.TargetLocaleId {
					targetTranslation = def.Translation
					break
				}
			}

			// Only suggest if we have a translation for the target locale
			if targetTranslation != "" {
				suggestions = append(suggestions, &pb.TerminologySuggestion{
					Terminology:       converter.TerminologyToProto(&term),
					TargetTranslation: targetTranslation,
					MatchedWords:      matchedWords,
				})
			}
		}
	}

	return connect.NewResponse(&pb.SuggestTerminologiesResponse{
		Suggestions: suggestions,
	}), nil
}

// tokenize splits text into lowercase words, removing punctuation
func tokenize(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Replace common punctuation with spaces
	replacer := strings.NewReplacer(
		".", " ",
		",", " ",
		"!", " ",
		"?", " ",
		";", " ",
		":", " ",
		"-", " ",
		"_", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		"\"", " ",
		"'", " ",
	)
	text = replacer.Replace(text)

	// Split into words
	words := strings.Fields(text)

	return words
}

// findMatches checks if the terminology term appears in the source words
// Returns the matched words from the source text
func findMatches(term string, sourceWords []string) []string {
	termLower := strings.ToLower(term)
	var matches []string

	// Check for exact word match
	for _, word := range sourceWords {
		if word == termLower {
			matches = append(matches, word)
		}
	}

	// Check for partial matches (term contains multiple words)
	termWords := strings.Fields(termLower)
	if len(termWords) > 1 {
		// Check if all words of the term appear in sequence in source
		for i := 0; i <= len(sourceWords)-len(termWords); i++ {
			allMatch := true
			for j, termWord := range termWords {
				if sourceWords[i+j] != termWord {
					allMatch = false
					break
				}
			}
			if allMatch {
				matches = append(matches, strings.Join(sourceWords[i:i+len(termWords)], " "))
			}
		}
	}

	return matches
}
