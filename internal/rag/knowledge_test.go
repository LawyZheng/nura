package rag

import (
	"path/filepath"
	"runtime"
	"testing"
)

func knowledgeDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "knowledge")
}

func TestKnowledgeStore_LoadFromDir(t *testing.T) {
	ks, err := LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if ks == nil {
		t.Fatal("expected non-nil KnowledgeStore")
	}
}

func TestKnowledgeStore_Search_HighRisk_Medication(t *testing.T) {
	ks, err := LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatal(err)
	}

	results := ks.Search("奥美拉唑能不能停")
	if len(results) == 0 {
		t.Fatal("expected results for medication query, got none")
	}
	hasMedSource := false
	for _, r := range results {
		if r.Source == "HP根治四联疗法常用药物库" {
			hasMedSource = true
		}
	}
	if !hasMedSource {
		t.Errorf("expected medication source, got sources: %v", results)
	}
}

func TestKnowledgeStore_Search_HighRisk_Eradication(t *testing.T) {
	ks, err := LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatal(err)
	}

	results := ks.Search("HP根治失败怎么办")
	if len(results) == 0 {
		t.Fatal("expected results for eradication query, got none")
	}
}

func TestKnowledgeStore_Search_MediumRisk_Diet(t *testing.T) {
	ks, err := LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatal(err)
	}

	results := ks.Search("能不能吃辣椒")
	if len(results) == 0 {
		t.Fatal("expected results for diet query, got none")
	}
	hasDietSource := false
	for _, r := range results {
		if r.Source == "溃疡患者饮食宜忌数据库" {
			hasDietSource = true
		}
	}
	if !hasDietSource {
		t.Errorf("expected diet source, got sources: %v", results)
	}
}

func TestKnowledgeStore_Search_LowRisk_NoResults(t *testing.T) {
	ks, err := LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatal(err)
	}

	results := ks.Search("HP呼气试验是什么意思")
	if len(results) != 0 {
		t.Errorf("expected no RAG results for low-risk query, got %d", len(results))
	}
}

func TestKnowledgeStore_Search_Contraindication(t *testing.T) {
	ks, err := LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatal(err)
	}

	results := ks.Search("阿莫西林有什么禁忌")
	if len(results) == 0 {
		t.Fatal("expected results for contraindication query, got none")
	}
	hasContraindication := false
	for _, r := range results {
		if r.Section == "contraindications" {
			hasContraindication = true
		}
	}
	if !hasContraindication {
		t.Error("expected contraindications section in results")
	}
}
