package trend

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/store"
)

const trendSystemPrompt = `你是一位专业的消化性溃疡健康管理助手。你的任务是根据用户的症状记录、饮食记录和用药数据，
用通俗易懂的中文给出趋势解读和生活建议。

## 规则
- 只基于用户提供的数据和检索到的指南内容回答
- 不做诊断，不建议具体药物和剂量调整
- 发现异常趋势（如黑便、剧痛、体重骤降）时强烈建议立即就医
- 关联饮食和用药数据分析症状变化的可能原因
- 追踪 HP 根治疗程进度，鼓励坚持
- 回答简洁，控制在 200 字以内
- 使用普通人能理解的语言，避免专业术语

## 紧急预警信号
- 黑便或血便 → 可能消化道出血，需立即就医
- 剧烈腹痛伴腹肌紧张 → 可能穿孔，需急诊
- 反复呕吐无法进食 → 可能幽门梗阻，需就医`

type InsightResult struct {
	Insight    *model.AIInsight `json:"insight"`
	Disclaimer string           `json:"disclaimer"`
}

type Service struct {
	llm   providers.LLMProvider
	guard *policy.OutputGuard
	store *store.Store
}

func NewService(llm providers.LLMProvider, s *store.Store) *Service {
	return &Service{
		llm:   llm,
		guard: policy.NewOutputGuard(),
		store: s,
	}
}

func (s *Service) GenerateInsight(ctx context.Context, patientID int, days int) (*InsightResult, error) {
	end := time.Now()
	start := end.AddDate(0, 0, -days)

	symptoms, _ := s.store.ListSymptomLogsByDateRange(patientID, start, end)
	meals, _ := s.store.ListMealLogsByDateRange(patientID, start, end)
	meds, _ := s.store.ListActiveMedications(patientID)

	userPrompt := buildTrendPrompt(symptoms, meals, meds, days)

	resp, err := s.llm.Complete(ctx, providers.CompletionRequest{
		SystemPrompt: trendSystemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.1,
	})
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	output := policy.AgentOutput{
		Content:    resp.Content,
		Confidence: "medium",
		RiskLevel:  policy.RiskMedium,
	}
	guarded, err := s.guard.GuardOutput(ctx, output)
	if err != nil {
		return nil, fmt.Errorf("guard output: %w", err)
	}

	insightType := fmt.Sprintf("trend_%dd", days)
	insight := &model.AIInsight{
		PatientID:      patientID,
		InsightType:    insightType,
		Content:        guarded.Content,
		DataRangeStart: start.Format("2006-01-02"),
		DataRangeEnd:   end.Format("2006-01-02"),
	}

	if _, err := s.store.InsertAIInsight(insight); err != nil {
		return nil, fmt.Errorf("insert insight: %w", err)
	}

	return &InsightResult{
		Insight:    insight,
		Disclaimer: guarded.Disclaimer,
	}, nil
}

func buildTrendPrompt(symptoms []*model.SymptomLog, meals []*model.MealLog, meds []*model.Medication, days int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 用户症状数据（最近 %d 天）\n", days))
	if len(symptoms) == 0 {
		sb.WriteString("无记录\n")
	} else {
		sb.WriteString("| 日期 | 疼痛评分 | 位置 | 大便 | 伴随 |\n")
		sb.WriteString("|------|---------|------|------|------|\n")
		for _, s := range symptoms {
			score := "-"
			if s.PainScore != nil {
				score = fmt.Sprintf("%d", *s.PainScore)
			}
			var companion []string
			if s.Bloating {
				companion = append(companion, "腹胀")
			}
			if s.Nausea {
				companion = append(companion, "恶心")
			}
			if s.AcidReflux {
				companion = append(companion, "反酸")
			}
			comp := "-"
			if len(companion) > 0 {
				comp = strings.Join(companion, "、")
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				s.RecordedAt.Format("01-02"), score, s.PainLocation, s.StoolColor, comp))
		}
	}

	sb.WriteString("\n## 饮食记录\n")
	if len(meals) == 0 {
		sb.WriteString("无记录\n")
	} else {
		for _, m := range meals {
			irritant := ""
			if m.HasIrritant {
				irritant = fmt.Sprintf(" [刺激性：%s]", m.IrritantDetail)
			}
			sb.WriteString(fmt.Sprintf("- %s %s: %s%s\n",
				m.RecordedAt.Format("01-02"), m.MealType, m.Content, irritant))
		}
	}

	sb.WriteString("\n## 用药情况（含疗程进度）\n")
	if len(meds) == 0 {
		sb.WriteString("无活跃用药\n")
	} else {
		for _, m := range meds {
			course := ""
			if m.CourseStart != "" && m.CourseEnd != "" {
				course = fmt.Sprintf(" (%s ~ %s)", m.CourseStart, m.CourseEnd)
			}
			sb.WriteString(fmt.Sprintf("- %s %s %s%s\n", m.Name, m.Dosage, m.Frequency, course))
		}
	}

	sb.WriteString("\n请分析以上数据的趋势，给出解读和建议。特别关注：\n")
	sb.WriteString("1. 症状评分是否在改善\n")
	sb.WriteString("2. 饮食中是否有刺激性食物与症状加重的关联\n")
	sb.WriteString("3. 用药依从性和疗程完成度\n")

	return sb.String()
}
