// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"archive/zip"
	"embed"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed docs_embedded/*.md
var docsFS embed.FS

//go:embed installer_scripts/install.sh
var installSh string

//go:embed installer_scripts/install.ps1
var installPs1 string

//go:embed vscode_embedded/* vscode_embedded/icons/* vscode_embedded/snippets/* vscode_embedded/syntaxes/*
var vscodeFS embed.FS

// DocTopic represents a documentation section.
type DocTopic struct {
	ID         string `json:"id"`
	Category   string `json:"category"`
	CatTitleRU string `json:"cat_title_ru"`
	CatTitleEN string `json:"cat_title_en"`
	TitleRU    string `json:"title_ru"`
	TitleEN    string `json:"title_en"`
	DescRU     string `json:"desc_ru"`
	DescEN     string `json:"desc_en"`
	Icon       string `json:"icon"`
}

var docTopics = []DocTopic{
	{
		ID:         "handbook",
		Category:   "getting_started",
		CatTitleRU: "Начало работы",
		CatTitleEN: "Getting Started",
		TitleRU:    "Руководство и учебник",
		TitleEN:    "Handbook & Tutorial",
		DescRU:     "Полное пошаговое руководство по изучению языка Alca от основ до ООП",
		DescEN:     "Comprehensive step-by-step tutorial covering Alca from basics to advanced OOP",
		Icon:       "book",
	},
	{
		ID:         "cli",
		Category:   "getting_started",
		CatTitleRU: "Начало работы",
		CatTitleEN: "Getting Started",
		TitleRU:    "Интерфейс командной строки",
		TitleEN:    "CLI Reference",
		DescRU:     "Команды компилятора alca, запуск, сборка, флаги и форматирование",
		DescEN:     "Compiler alca flags, run, build, test, and formatting options",
		Icon:       "terminal",
	},
	{
		ID:         "editors",
		Category:   "getting_started",
		CatTitleRU: "Начало работы",
		CatTitleEN: "Getting Started",
		TitleRU:    "Редакторы и IDE",
		TitleEN:    "Editors & IDE Setup",
		DescRU:     "Плагины и подсветка синтаксиса для VS Code, Vim, Neovim и Sublime Text",
		DescEN:     "Syntax highlighting and extensions for VS Code, Vim, Neovim, and Sublime",
		Icon:       "edit",
	},
	{
		ID:         "language",
		Category:   "reference",
		CatTitleRU: "Справочники",
		CatTitleEN: "Reference Manuals",
		TitleRU:    "Спецификация языка",
		TitleEN:    "Language Reference",
		DescRU:     "Формальный синтаксис, семантика, операторы, срезы, сопоставление с образцом",
		DescEN:     "Formal syntax, operators, slices, pattern matching, functions, and classes",
		Icon:       "code",
	},
	{
		ID:         "stdlib",
		Category:   "reference",
		CatTitleRU: "Справочники",
		CatTitleEN: "Reference Manuals",
		TitleRU:    "Стандартная библиотека",
		TitleEN:    "Standard Library Reference",
		DescRU:     "Исчерпывающий справочник встроенных модулей: http, json, crypto, io, regex, ffi",
		DescEN:     "Complete reference of built-in modules: http, json, crypto, io, regex, ffi",
		Icon:       "library",
	},
	{
		ID:         "apm",
		Category:   "reference",
		CatTitleRU: "Справочники",
		CatTitleEN: "Reference Manuals",
		TitleRU:    "Пакетный менеджер APM",
		TitleEN:    "APM Package Manager",
		DescRU:     "Управление библиотеками, манифест alca.toml, alca.lock и публикация в реестр",
		DescEN:     "Package dependencies, alca.toml, alca.lock, lockfiles, and registry publishing",
		Icon:       "box",
	},
	{
		ID:         "architecture",
		Category:   "advanced",
		CatTitleRU: "Продвинутый уровень",
		CatTitleEN: "Advanced Topics",
		TitleRU:    "Архитектура и ВМ",
		TitleEN:    "Architecture & VM",
		DescRU:     "Устройство компилятора, байткод-машины, модель памяти и сборщик мусора",
		DescEN:     "Compiler pipeline, bytecode virtual machine, memory model, and GC",
		Icon:       "cpu",
	},
	{
		ID:         "cross-compilation",
		Category:   "advanced",
		CatTitleRU: "Продвинутый уровень",
		CatTitleEN: "Advanced Topics",
		TitleRU:    "Кросс-компиляция",
		TitleEN:    "Cross-Compilation",
		DescRU:     "Сборка статических бинарников под Linux, Windows и macOS без внешних SDK",
		DescEN:     "Targeting Linux, Windows, and macOS without toolchain dependencies",
		Icon:       "globe",
	},
	{
		ID:         "bug-report",
		Category:   "advanced",
		CatTitleRU: "Продвинутый уровень",
		CatTitleEN: "Advanced Topics",
		TitleRU:    "Сообщения об ошибках",
		TitleEN:    "Bug Reporting",
		DescRU:     "Инструкция по созданию воспроизводимых отчётов об ошибках компилятора",
		DescEN:     "Filing actionable compiler and runtime bug reports",
		Icon:       "alert",
	},
}

// handleDocsList returns the list of all available documentation topics.
func (s *Server) handleDocsList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"topics": docTopics,
	})
}

// handleDocContent returns the content of a single documentation document.
func (s *Server) handleDocContent(w http.ResponseWriter, r *http.Request) {
	topic := strings.ToLower(r.PathValue("topic"))
	lang := strings.ToUpper(r.URL.Query().Get("lang"))
	if lang != "EN" && lang != "RU" {
		lang = "RU"
	}

	var found *DocTopic
	for i := range docTopics {
		if strings.EqualFold(docTopics[i].ID, topic) {
			found = &docTopics[i]
			break
		}
	}
	if found == nil {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("documentation topic '%s' not found", topic))
		return
	}

	fileName := fmt.Sprintf("%s_%s.md", found.ID, lang)

	// Try reading from disk docs/ directory first
	var data []byte
	var err error
	for _, dir := range []string{"docs", "../docs", "../../docs"} {
		p := filepath.Join(dir, fileName)
		if b, readErr := os.ReadFile(p); readErr == nil {
			data = b
			break
		}
	}

	// Fallback to embedded filesystem
	if len(data) == 0 {
		data, err = docsFS.ReadFile("docs_embedded/" + fileName)
		if err != nil {
			otherLang := "EN"
			if lang == "EN" {
				otherLang = "RU"
			}
			fallbackFile := fmt.Sprintf("%s_%s.md", found.ID, otherLang)
			data, err = docsFS.ReadFile("docs_embedded/" + fallbackFile)
		}
	}

	if err != nil || len(data) == 0 {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("content for topic '%s' (%s) not found", topic, lang))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	title := found.TitleRU
	if lang == "EN" {
		title = found.TitleEN
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      found.ID,
		"lang":    strings.ToLower(lang),
		"title":   title,
		"content": string(data),
	})
}

// handleInstallSh serves the Linux/macOS bash installer.
func (s *Server) handleInstallSh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="install.sh"`)
	_, _ = w.Write([]byte(installSh))
}

// handleInstallPs1 serves the Windows PowerShell installer.
func (s *Server) handleInstallPs1(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="install.ps1"`)
	_, _ = w.Write([]byte(installPs1))
}

// handleVscodeZip streams the VS Code extension bundle as an in-memory zip archive.
func (s *Server) handleVscodeZip(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="vscode-alca.zip"`)
	zw := zip.NewWriter(w)
	defer zw.Close()

	_ = fs.WalkDir(vscodeFS, "vscode_embedded", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("vscode_embedded", path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		data, err := vscodeFS.ReadFile(path)
		if err != nil {
			return nil
		}
		f, err := zw.Create(rel)
		if err != nil {
			return nil
		}
		_, _ = f.Write(data)
		return nil
	})
}
