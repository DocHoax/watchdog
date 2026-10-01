package governance

import (
	"errors"
	"strings"
	"testing"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSelector_Lexer(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTypes []TokenType
		wantErr   bool
	}{
		{
			name:  "basic equality",
			input: `env == "production"`,
			wantTypes: []TokenType{
				TokenIdent, TokenEq, TokenString, TokenEOF,
			},
		},
		{
			name:  "logical AND with IN list",
			input: `env == "prod" AND region in ["us-east-1", "us-west-2"]`,
			wantTypes: []TokenType{
				TokenIdent, TokenEq, TokenString, TokenAnd, TokenIdent, TokenIn,
				TokenLBracket, TokenString, TokenComma, TokenString, TokenRBracket, TokenEOF,
			},
		},
		{
			name:  "NOT EXISTS and relational",
			input: `NOT EXISTS tags.deprecated && cpu_cores >= 8`,
			wantTypes: []TokenType{
				TokenNotExists, TokenIdent, TokenAnd, TokenIdent, TokenGte, TokenNumber, TokenEOF,
			},
		},
		{
			name:  "NOT IN operator variant",
			input: `tier not in ["deprecated", "staging"]`,
			wantTypes: []TokenType{
				TokenIdent, TokenNotIn, TokenLBracket, TokenString, TokenComma, TokenString, TokenRBracket, TokenEOF,
			},
		},
		{
			name:    "unterminated string",
			input:   `env == "production`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tokens, err := lexer.Tokenize()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Tokenize() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(tt.wantTypes) > 0 {
				if len(tokens) != len(tt.wantTypes) {
					t.Fatalf("expected %d tokens, got %d", len(tt.wantTypes), len(tokens))
				}
				for i, wantType := range tt.wantTypes {
					if tokens[i].Type != wantType {
						t.Errorf("token %d: expected type %v, got %v (%q)", i, wantType, tokens[i].Type, tokens[i].Literal)
					}
				}
			}
		})
	}
}

func TestSelector_Evaluation(t *testing.T) {
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:      "node-srv-01",
			Hostname:    "srv-01.us-east.company.internal",
			OS:          "linux",
			Platform:    "ubuntu",
			Arch:        "amd64",
			CPUCores:    16,
			TotalMemory: 64 * 1024 * 1024 * 1024, // 64 GB
			Tags: map[string]string{
				"cluster": "k8s-prod-01",
				"tier":    "database",
			},
		},
		Status: model.NodeStatusHealthy,
		Metadata: map[string]string{
			"env":              "production",
			"region":           "us-east-1",
			"custom.compliance": "pci-dss",
		},
	}

	ownership := &model.NodeOwnershipMetadata{
		OwnerTeam:           "Infrastructure Core",
		ContactEmail:        "infra@company.internal",
		Environment:         "production",
		Region:              "us-east-1",
		DataClassification:  "pci-dss",
		CostCenter:          "CC-1001",
		BusinessCriticality: model.CriticalityMissionCritical,
		Lifecycle:           model.LifecycleActive,
	}

	groupIDs := []string{"grp-root", "grp-prod", "grp-db"}
	groupPaths := []string{"/grp-root", "/grp-root/grp-prod", "/grp-root/grp-prod/grp-db"}

	tests := []struct {
		name      string
		selector  string
		wantMatch bool
		wantErr   bool
	}{
		{
			name:      "empty selector matches all",
			selector:  "",
			wantMatch: true,
		},
		{
			name:      "wildcard matches all",
			selector:  "*",
			wantMatch: true,
		},
		{
			name:      "simple equality matching",
			selector:  `environment == "production"`,
			wantMatch: true,
		},
		{
			name:      "simple equality non-matching",
			selector:  `environment == "staging"`,
			wantMatch: false,
		},
		{
			name:      "hostname matching",
			selector:  `hostname == "srv-01.us-east.company.internal"`,
			wantMatch: true,
		},
		{
			name:      "numeric comparison on CPU cores",
			selector:  `cpu_cores >= 8 AND cpu_cores <= 32`,
			wantMatch: true,
		},
		{
			name:      "numeric comparison failure",
			selector:  `cpu_cores > 32`,
			wantMatch: false,
		},
		{
			name:      "memory gb calculation",
			selector:  `memory_gb >= 60`,
			wantMatch: true,
		},
		{
			name:      "set membership IN match",
			selector:  `region IN ["us-east-1", "eu-west-1"]`,
			wantMatch: true,
		},
		{
			name:      "set membership IN non-match",
			selector:  `region IN ["ap-southeast-1", "eu-central-1"]`,
			wantMatch: false,
		},
		{
			name:      "set membership NOT IN",
			selector:  `region NOT IN ["ap-southeast-1", "eu-central-1"]`,
			wantMatch: true,
		},
		{
			name:      "group ID slice matching",
			selector:  `group_id == "grp-prod"`,
			wantMatch: true,
		},
		{
			name:      "group path matching",
			selector:  `group == "/grp-root/grp-prod/grp-db"`,
			wantMatch: true,
		},
		{
			name:      "tags prefix matching",
			selector:  `tags.cluster == "k8s-prod-01" AND tags.tier == "database"`,
			wantMatch: true,
		},
		{
			name:      "exists expression true",
			selector:  `EXISTS owner_team`,
			wantMatch: true,
		},
		{
			name:      "exists expression false",
			selector:  `EXISTS non_existent_prop`,
			wantMatch: false,
		},
		{
			name:      "not exists expression true",
			selector:  `NOT EXISTS tags.deprecated`,
			wantMatch: true,
		},
		{
			name:      "complex boolean logic with parenthesis",
			selector:  `(environment == "production" OR environment == "staging") AND (criticality == "mission_critical" OR criticality == "high") AND cpu_cores >= 16`,
			wantMatch: true,
		},
		{
			name:      "business criticality match",
			selector:  `business_criticality == "mission_critical"`,
			wantMatch: true,
		},
		{
			name:      "negation expression",
			selector:  `NOT (status == "critical")`,
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, err := MatchesNode(tt.selector, node, ownership, groupIDs, groupPaths)
			if (err != nil) != tt.wantErr {
				t.Fatalf("MatchesNode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if matched != tt.wantMatch {
				t.Errorf("MatchesNode(%q) = %v, want %v", tt.selector, matched, tt.wantMatch)
			}
		})
	}
}

func TestSelector_ErrorsAndLimits(t *testing.T) {
	t.Run("exceeds max length", func(t *testing.T) {
		longQuery := "env == 'prod' AND " + strings.Repeat("region == 'us-east-1' AND ", 100) + "status == 'healthy'"
		_, err := ParseSelector(longQuery)
		if !errors.Is(err, ErrSelectorTooLong) && !errors.Is(err, ErrInvalidSelector) {
			t.Errorf("expected ErrSelectorTooLong or ErrInvalidSelector, got %v", err)
		}
	})

	t.Run("exceeds max depth", func(t *testing.T) {
		deepQuery := strings.Repeat("((((((", 3) + "env == 'prod'" + strings.Repeat("))))))", 3)
		_, err := ParseSelector(deepQuery)
		if !errors.Is(err, ErrSelectorMaxDepthExceeded) {
			t.Errorf("expected ErrSelectorMaxDepthExceeded, got %v", err)
		}
	})

	t.Run("syntax error missing bracket", func(t *testing.T) {
		_, err := ParseSelector(`region IN ["us-east-1", "us-west-2"`)
		if err == nil {
			t.Errorf("expected syntax error on unclosed bracket")
		}
	})

	t.Run("syntax error invalid operator", func(t *testing.T) {
		_, err := ParseSelector(`region ~ "us-east"`)
		if err == nil {
			t.Errorf("expected syntax error on invalid operator")
		}
	})
}
