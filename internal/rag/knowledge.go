package rag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SearchResult struct {
	Source  string `json:"source"`
	Section string `json:"section"`
	Content string `json:"content"`
}

type KnowledgeStore struct {
	medications *medicationKB
	diet        *dietKB
}

var medicationKeywords = []string{
	"用药", "停药", "换药", "剂量", "处方", "药物",
	"四联", "三联", "根治", "耐药", "抗生素",
	"奥美拉唑", "雷贝拉唑", "阿莫西林", "克拉霉素", "甲硝唑", "左氧氟沙星",
	"铋剂", "枸橼酸铋钾", "铝碳酸镁",
	"禁忌",
}

var dietKeywords = []string{
	"饮食", "吃什么", "能不能吃", "忌口", "能吃", "不能吃",
	"辣椒", "咖啡", "酒", "茶",
}

func LoadFromDir(dir string) (*KnowledgeStore, error) {
	ks := &KnowledgeStore{}

	medPath := filepath.Join(dir, "medications", "duodenal_ulcer_drugs.json")
	med, err := loadMedications(medPath)
	if err != nil {
		return nil, fmt.Errorf("load medications: %w", err)
	}
	ks.medications = med

	dietPath := filepath.Join(dir, "diet", "duodenal_ulcer_diet.json")
	d, err := loadDiet(dietPath)
	if err != nil {
		return nil, fmt.Errorf("load diet: %w", err)
	}
	ks.diet = d

	return ks, nil
}

func (ks *KnowledgeStore) Search(message string) []SearchResult {
	msg := strings.ToLower(message)
	var results []SearchResult

	if matchesAny(msg, medicationKeywords) {
		results = append(results, ks.medications.search(msg)...)
	}
	if matchesAny(msg, dietKeywords) {
		results = append(results, ks.diet.search(msg)...)
	}

	return results
}

func matchesAny(msg string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(msg, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

type medicationKB struct {
	QuadrupleTherapy struct {
		Description string `json:"description"`
		Duration    string `json:"duration_days"`
		Components  []struct {
			Category string `json:"category"`
			NameCN   string `json:"name_cn"`
			NameEN   string `json:"name_en"`
			Dosage   string `json:"dosage"`
			Freq     string `json:"frequency"`
			Role     string `json:"role"`
		} `json:"components"`
	} `json:"quadruple_therapy"`
	MaintenanceTherapy struct {
		Description string `json:"description"`
		Duration    string `json:"duration_weeks"`
		Medications []struct {
			NameCN string `json:"name_cn"`
			Dosage string `json:"dosage"`
			Freq   string `json:"frequency"`
			Note   string `json:"note"`
		} `json:"medications"`
	} `json:"maintenance_therapy"`
	Contraindications []struct {
		Drug             string `json:"drug"`
		Contraindication string `json:"contraindication"`
	} `json:"contraindications"`
}

func loadMedications(path string) (*medicationKB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var kb medicationKB
	if err := json.Unmarshal(data, &kb); err != nil {
		return nil, err
	}
	return &kb, nil
}

func (kb *medicationKB) search(msg string) []SearchResult {
	const source = "HP根治四联疗法常用药物库"
	var results []SearchResult

	if matchesAny(msg, []string{"四联", "根治", "抗生素", "耐药"}) {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s（疗程 %s 天）：\n", kb.QuadrupleTherapy.Description, kb.QuadrupleTherapy.Duration))
		for _, c := range kb.QuadrupleTherapy.Components {
			sb.WriteString(fmt.Sprintf("- %s（%s）%s %s — %s\n", c.NameCN, c.NameEN, c.Dosage, c.Freq, c.Role))
		}
		results = append(results, SearchResult{Source: source, Section: "quadruple_therapy", Content: sb.String()})
	}

	for _, c := range kb.QuadrupleTherapy.Components {
		if strings.Contains(msg, strings.ToLower(c.NameCN)) {
			content := fmt.Sprintf("%s（%s）%s %s — %s", c.NameCN, c.NameEN, c.Dosage, c.Freq, c.Role)
			results = append(results, SearchResult{Source: source, Section: "quadruple_therapy", Content: content})
		}
	}

	if matchesAny(msg, []string{"停药", "维持", "愈合"}) {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s（维持 %s 周）：\n", kb.MaintenanceTherapy.Description, kb.MaintenanceTherapy.Duration))
		for _, m := range kb.MaintenanceTherapy.Medications {
			sb.WriteString(fmt.Sprintf("- %s %s %s — %s\n", m.NameCN, m.Dosage, m.Freq, m.Note))
		}
		results = append(results, SearchResult{Source: source, Section: "maintenance_therapy", Content: sb.String()})
	}

	if matchesAny(msg, []string{"禁忌", "过敏", "不良反应"}) {
		var sb strings.Builder
		sb.WriteString("药物禁忌：\n")
		for _, ci := range kb.Contraindications {
			sb.WriteString(fmt.Sprintf("- %s：%s\n", ci.Drug, ci.Contraindication))
		}
		results = append(results, SearchResult{Source: source, Section: "contraindications", Content: sb.String()})
	}

	for _, ci := range kb.Contraindications {
		if strings.Contains(msg, strings.ToLower(ci.Drug)) {
			content := fmt.Sprintf("%s：%s", ci.Drug, ci.Contraindication)
			results = append(results, SearchResult{Source: source, Section: "contraindications", Content: content})
		}
	}

	return dedup(results)
}

type dietKB struct {
	TriggerFoods []struct {
		Category  string   `json:"category"`
		Items     []string `json:"items"`
		Reason    string   `json:"reason"`
		RiskLevel string   `json:"risk_level"`
	} `json:"trigger_foods"`
	SafeFoods []struct {
		Category string   `json:"category"`
		Items    []string `json:"items"`
		Note     string   `json:"note"`
	} `json:"safe_foods"`
	EatingHabits []string `json:"eating_habits"`
}

func loadDiet(path string) (*dietKB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var kb dietKB
	if err := json.Unmarshal(data, &kb); err != nil {
		return nil, err
	}
	return &kb, nil
}

func (kb *dietKB) search(msg string) []SearchResult {
	const source = "溃疡患者饮食宜忌数据库"
	var results []SearchResult

	for _, cat := range kb.TriggerFoods {
		for _, item := range cat.Items {
			if strings.Contains(msg, strings.ToLower(item)) {
				content := fmt.Sprintf("【%s类 — 应避免】%s：%s（风险：%s）", cat.Category, item, cat.Reason, cat.RiskLevel)
				results = append(results, SearchResult{Source: source, Section: "trigger_foods", Content: content})
			}
		}
	}

	for _, cat := range kb.SafeFoods {
		for _, item := range cat.Items {
			if strings.Contains(msg, strings.ToLower(item)) {
				content := fmt.Sprintf("【%s类 — 推荐】%s（%s）", cat.Category, item, cat.Note)
				results = append(results, SearchResult{Source: source, Section: "safe_foods", Content: content})
			}
		}
	}

	if matchesAny(msg, []string{"饮食", "吃什么", "忌口"}) && len(results) == 0 {
		var sb strings.Builder
		sb.WriteString("饮食建议：\n")
		for _, h := range kb.EatingHabits {
			sb.WriteString(fmt.Sprintf("- %s\n", h))
		}
		results = append(results, SearchResult{Source: source, Section: "eating_habits", Content: sb.String()})
	}

	return results
}

func dedup(results []SearchResult) []SearchResult {
	seen := make(map[string]bool)
	var out []SearchResult
	for _, r := range results {
		key := r.Source + "|" + r.Section + "|" + r.Content
		if !seen[key] {
			seen[key] = true
			out = append(out, r)
		}
	}
	return out
}
