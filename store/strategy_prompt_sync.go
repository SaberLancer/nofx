package store

import (
	"fmt"
	"strings"
	"time"
)

// SyncPromptSectionsResult summarizes a prompt-section sync run.
type SyncPromptSectionsResult struct {
	Total     int
	Updated   int
	Skipped   int
	Errors    []string
	UpdatedID []string
}

// SyncPromptSectionsFromDefaults updates reduce/exit/decision_process on all AI
// trading strategies to the current default templates for each strategy language.
// Other prompt sections and custom_prompt are preserved.
func (s *StrategyStore) SyncPromptSectionsFromDefaults() (*SyncPromptSectionsResult, error) {
	var strategies []Strategy
	if err := s.db.Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("list strategies: %w", err)
	}

	result := &SyncPromptSectionsResult{Total: len(strategies)}

	for i := range strategies {
		st := &strategies[i]
		cfg, err := st.ParseConfig()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: parse config: %v", st.ID, err))
			continue
		}

		strategyType := strings.TrimSpace(cfg.StrategyType)
		if strategyType == "" {
			strategyType = "ai_trading"
		}
		if strategyType == "grid_trading" {
			result.Skipped++
			continue
		}

		lang := strings.TrimSpace(cfg.Language)
		if lang == "" {
			lang = "en"
		}
		if lang != "zh" && lang != "en" {
			lang = "en"
		}

		defaults := GetDefaultStrategyConfig(lang)
		cfg.PromptSections.ReduceStandards = defaults.PromptSections.ReduceStandards
		cfg.PromptSections.ExitStandards = defaults.PromptSections.ExitStandards
		cfg.PromptSections.DecisionProcess = defaults.PromptSections.DecisionProcess

		if err := st.SetConfig(cfg); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: serialize config: %v", st.ID, err))
			continue
		}

		if err := s.db.Model(&Strategy{}).Where("id = ?", st.ID).Updates(map[string]interface{}{
			"config":     st.Config,
			"updated_at": time.Now().UTC(),
		}).Error; err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: save: %v", st.ID, err))
			continue
		}

		result.Updated++
		result.UpdatedID = append(result.UpdatedID, st.ID)
	}

	return result, nil
}
