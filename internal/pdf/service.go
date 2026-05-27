package pdf

import (
	"bytes"
	"fmt"
	"time"

	"github.com/LawyZheng/nura/internal/pdf/fonts"
	"github.com/LawyZheng/nura/internal/store"

	"github.com/go-pdf/fpdf"
)

type Service struct {
	store *store.Store
}

func NewService(s *store.Store) *Service {
	return &Service{store: s}
}

func (s *Service) GenerateSummary(patientID int) ([]byte, error) {
	profile, err := s.store.GetPatientProfile(patientID)
	if err != nil {
		return nil, fmt.Errorf("get patient: %w", err)
	}

	diagnoses, _ := s.store.ListDiagnoses(patientID)
	meds, _ := s.store.ListActiveMedications(patientID)
	abnormals, _ := s.store.GetAbnormalIndicators(patientID)
	now := time.Now()
	weekAgo := now.Add(-7 * 24 * time.Hour)
	symptoms, _ := s.store.ListSymptomLogsByDateRange(patientID, weekAgo, now)
	insight, _ := s.store.GetLatestAIInsight(patientID, "trend_7d")

	fontBytes, err := fonts.FS.ReadFile("NotoSansSC-Regular.ttf")
	if err != nil {
		return nil, fmt.Errorf("read font: %w", err)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("noto", "", fontBytes)
	pdf.SetFont("noto", "", 12)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddPage()

	pageW, _ := pdf.GetPageSize()
	marginL, _, marginR, _ := pdf.GetMargins()
	contentW := pageW - marginL - marginR

	heading := func(text string) {
		pdf.SetFont("noto", "", 14)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(contentW, 8, text, "", 1, "L", true, 0, "")
		pdf.Ln(2)
		pdf.SetFont("noto", "", 10)
	}

	cell := func(w float64, text string) {
		pdf.CellFormat(w, 6, text, "1", 0, "L", false, 0, "")
	}

	pdf.SetFont("noto", "", 18)
	title := fmt.Sprintf("复诊摘要 — %s", profile.Name)
	pdf.CellFormat(contentW, 12, title, "", 1, "C", false, 0, "")
	pdf.SetFont("noto", "", 9)
	pdf.CellFormat(contentW, 6, fmt.Sprintf("生成日期：%s", now.Format("2006-01-02 15:04")), "", 1, "C", false, 0, "")
	pdf.Ln(6)

	heading("基本信息")
	gender := profile.Gender
	if gender == "male" {
		gender = "男"
	} else if gender == "female" {
		gender = "女"
	}
	pdf.CellFormat(contentW, 6, fmt.Sprintf("姓名：%s　　性别：%s　　出生日期：%s", profile.Name, gender, profile.BirthDate), "", 1, "L", false, 0, "")
	if len(profile.Allergies) > 0 {
		allergies := ""
		for i, a := range profile.Allergies {
			if i > 0 {
				allergies += "、"
			}
			allergies += a
		}
		pdf.CellFormat(contentW, 6, fmt.Sprintf("过敏史：%s", allergies), "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)

	if len(diagnoses) > 0 {
		heading("诊断记录")
		colW := []float64{contentW * 0.2, contentW * 0.4, contentW * 0.4}
		cell(colW[0], "日期")
		cell(colW[1], "诊断")
		cell(colW[2], "备注")
		pdf.Ln(-1)
		for _, d := range diagnoses {
			cell(colW[0], d.DiagnosisDate)
			cell(colW[1], d.Condition)
			note := d.Note
			if note == "" {
				note = "-"
			}
			cell(colW[2], note)
			pdf.Ln(-1)
		}
		pdf.Ln(4)
	}

	if len(meds) > 0 {
		heading("当前用药")
		colW := []float64{contentW * 0.25, contentW * 0.2, contentW * 0.2, contentW * 0.35}
		cell(colW[0], "药名")
		cell(colW[1], "剂量")
		cell(colW[2], "频率")
		cell(colW[3], "疗程")
		pdf.Ln(-1)
		for _, m := range meds {
			cell(colW[0], m.Name)
			cell(colW[1], m.Dosage)
			cell(colW[2], m.Frequency)
			course := "-"
			if m.CourseEnd != "" {
				course = fmt.Sprintf("%s 至 %s", m.CourseStart, m.CourseEnd)
			}
			cell(colW[3], course)
			pdf.Ln(-1)
		}
		pdf.Ln(4)
	}

	if len(abnormals) > 0 {
		heading("异常指标")
		colW := []float64{contentW * 0.25, contentW * 0.15, contentW * 0.25, contentW * 0.15, contentW * 0.2}
		cell(colW[0], "指标")
		cell(colW[1], "值")
		cell(colW[2], "参考范围")
		cell(colW[3], "日期")
		cell(colW[4], "状态")
		pdf.Ln(-1)
		for _, ind := range abnormals {
			name := ind.IndicatorNameCN
			if name == "" {
				name = ind.IndicatorName
			}
			cell(colW[0], name)
			cell(colW[1], fmt.Sprintf("%s %s", ind.Value, ind.Unit))
			ref := ""
			if ind.ReferenceLow != nil {
				ref += fmt.Sprintf("%.1f", *ind.ReferenceLow)
			}
			if ind.ReferenceLow != nil && ind.ReferenceHigh != nil {
				ref += " - "
			}
			if ind.ReferenceHigh != nil {
				ref += fmt.Sprintf("%.1f", *ind.ReferenceHigh)
			}
			cell(colW[2], ref)
			cell(colW[3], ind.MeasuredAt)
			dir := ind.AbnormalDirection
			switch dir {
			case "high":
				dir = "偏高"
			case "low":
				dir = "偏低"
			case "positive":
				dir = "阳性"
			}
			cell(colW[4], dir)
			pdf.Ln(-1)
		}
		pdf.Ln(4)
	}

	if len(symptoms) > 0 {
		heading("近7天症状记录")
		colW := []float64{contentW * 0.2, contentW * 0.15, contentW * 0.25, contentW * 0.4}
		cell(colW[0], "日期")
		cell(colW[1], "疼痛评分")
		cell(colW[2], "位置")
		cell(colW[3], "备注")
		pdf.Ln(-1)
		for _, sym := range symptoms {
			cell(colW[0], sym.RecordedAt.Format("01-02 15:04"))
			score := "-"
			if sym.PainScore != nil {
				score = fmt.Sprintf("%d/10", *sym.PainScore)
			}
			cell(colW[1], score)
			loc := sym.PainLocation
			switch loc {
			case "upper_abdomen":
				loc = "上腹"
			case "lower_abdomen":
				loc = "下腹"
			case "epigastric":
				loc = "剑突下"
			}
			cell(colW[2], loc)
			note := sym.Note
			if note == "" {
				note = "-"
			}
			cell(colW[3], note)
			pdf.Ln(-1)
		}
		pdf.Ln(4)
	}

	if insight != nil && insight.Content != "" {
		heading("AI 趋势分析")
		pdf.SetFont("noto", "", 10)
		pdf.MultiCell(contentW, 5, insight.Content, "", "L", false)
		pdf.Ln(4)
	}

	pdf.Ln(6)
	pdf.SetFont("noto", "", 8)
	pdf.SetTextColor(180, 100, 0)
	disclaimer := "⚠ 本摘要由 AI 自动生成，仅供复诊参考，不构成医疗诊断或处方建议。如有疑问请咨询专业医生。"
	pdf.MultiCell(contentW, 4, disclaimer, "", "C", false)
	pdf.SetTextColor(150, 150, 150)
	pdf.CellFormat(contentW, 5, fmt.Sprintf("生成时间：%s · 知愈 Nura v0.4", now.Format("2006-01-02 15:04:05")), "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}
