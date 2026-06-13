package engine

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// This file is the coverage fixture for #4979 — detection-only registry records
// for the five C/C++ GUI / computer-vision / game frameworks whose detection
// signatures live in rules/cpp/frameworks/*.yaml (added by #4926, recorded by
// #4979). These frameworks are DETECTION-ONLY: the engine carries no deep
// component/state extractor for them, so the coverage cells they back are
// framework-specific = missing/not_applicable with the generic c-cpp substrate
// inherited. Each cite in the registry points at the YAML these tests load.
//
// Each framework family is proven by three cases, mirroring the
// happy-path / wrong-language-no-op / no-match-no-op fixture contract used by
// the rest of the c/c++ coverage work:
//
//   - happy path:           a representative C++ source string contains the
//                           framework's structural markers / include patterns
//                           declared in its detection YAML.
//   - wrong-language no-op:  the same detection signatures do NOT fire on a
//                           source from an unrelated language (Python here),
//                           guarding against cross-language misattribution.
//   - no-match no-op:        a plain C++ source that uses none of the framework
//                           matches none of the framework's markers.

// cppDetectionBlock is the minimal shape of a rules/cpp/frameworks/*.yaml file
// needed to read its detection signatures. Only the fields these fixtures
// assert on are decoded.
type cppDetectionBlock struct {
	Frameworks struct {
		Name      string `yaml:"name"`
		Category  string `yaml:"category"`
		Detection struct {
			IncludePatterns   []string `yaml:"include_patterns"`
			StructuralMarkers []string `yaml:"structural_markers"`
		} `yaml:"detection"`
	} `yaml:"frameworks"`
}

func loadCppFrameworkDetection(t *testing.T, file string) cppDetectionBlock {
	t.Helper()
	data, err := rulesFS.ReadFile("rules/cpp/frameworks/" + file)
	if err != nil {
		t.Fatalf("embedded detection YAML %q not found: %v", file, err)
	}
	var blk cppDetectionBlock
	if err := yaml.Unmarshal(data, &blk); err != nil {
		t.Fatalf("parsing %q: %v", file, err)
	}
	return blk
}

// anyMarkerMatches reports whether src contains any of the framework's
// structural markers or include patterns.
func anyMarkerMatches(blk cppDetectionBlock, src string) (string, bool) {
	sigs := append([]string{}, blk.Frameworks.Detection.StructuralMarkers...)
	sigs = append(sigs, blk.Frameworks.Detection.IncludePatterns...)
	for _, s := range sigs {
		if s == "" {
			continue
		}
		if strings.Contains(src, s) {
			return s, true
		}
	}
	return "", false
}

func TestCppGuiCvGameDetection(t *testing.T) {
	cases := []struct {
		file         string
		wantName     string
		wantCategory string
		// happy is a representative C++ source that uses the framework.
		happy string
		// wrongLang is a same-purpose source in an unrelated language; the
		// C/C++ structural markers must NOT appear in it.
		wrongLang string
		// noMatch is a plain C++ source that uses none of the framework.
		noMatch string
	}{
		{
			file:         "opencv.yaml",
			wantName:     "OpenCV",
			wantCategory: "computer_vision",
			happy: `#include <opencv2/opencv.hpp>
int main() {
    cv::Mat img = cv::imread("in.png");
    cv::imshow("w", img);
    cv::waitKey(0);
}`,
			// Python OpenCV bindings: cv2 module, no cv:: namespace / opencv2 includes.
			wrongLang: `import cv2
img = cv2.imread("in.png")
cv2.imshow("w", img)
cv2.waitKey(0)`,
			noMatch: `#include <vector>
int add(int a, int b) { return a + b; }`,
		},
		{
			file:         "dear_imgui.yaml",
			wantName:     "Dear ImGui",
			wantCategory: "immediate_mode_gui",
			happy: `#include "imgui.h"
void draw() {
    ImGui::Begin("hello");
    ImGui::Button("ok");
    ImGui::End();
}`,
			// pyimgui: imgui module, snake_case API, no ImGui:: C++ namespace.
			wrongLang: `import imgui
imgui.begin("hello")
imgui.button("ok")
imgui.end()`,
			noMatch: `#include <string>
std::string greet() { return "hi"; }`,
		},
		{
			file:         "cocos2d_x.yaml",
			wantName:     "Cocos2d-x",
			wantCategory: "game_engine_2d",
			happy: `#include "cocos2d.h"
USING_NS_CC;
class GameScene : public cocos2d::Layer {
    CREATE_FUNC(GameScene);
    CCSprite* hero;
};`,
			// Cocos Creator (TypeScript) component, not cocos2d-x C++.
			wrongLang: `import { _decorator, Component } from 'cc';
const { ccclass } = _decorator;
@ccclass('GameScene')
export class GameScene extends Component {}`,
			noMatch: `#include <cstdio>
void log() { printf("tick\n"); }`,
		},
		{
			file:         "juce.yaml",
			wantName:     "JUCE",
			wantCategory: "audio_gui_framework",
			happy: `#include <JuceHeader.h>
class Plugin : public AudioProcessor {
    void prepareToPlay(double, int) override {}
};`,
			// Web Audio API (JS) — different audio stack, no JUCE markers.
			wrongLang: `const ctx = new AudioContext();
const node = ctx.createGain();
node.connect(ctx.destination);`,
			noMatch: `#include <cmath>
double square(double x) { return x * x; }`,
		},
		{
			file:         "wxwidgets.yaml",
			wantName:     "wxWidgets",
			wantCategory: "gui_application",
			happy: `#include <wx/wx.h>
class MyApp : public wxApp {
public:
    bool OnInit() override { auto* f = new wxFrame(nullptr, wxID_ANY, "t"); f->Show(); return true; }
};
IMPLEMENT_APP(MyApp);`,
			// wxPython — Python bindings, no <wx/...> C++ includes / macros.
			wrongLang: `import wx
app = wx.App()
frame = wx.Frame(None, title="t")
frame.Show()
app.MainLoop()`,
			noMatch: `#include <iostream>
int main() { std::cout << "hi"; }`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.wantName, func(t *testing.T) {
			blk := loadCppFrameworkDetection(t, tc.file)

			// Sanity: the YAML declares the expected framework + category.
			if blk.Frameworks.Name != tc.wantName {
				t.Errorf("%s: name = %q, want %q", tc.file, blk.Frameworks.Name, tc.wantName)
			}
			if blk.Frameworks.Category != tc.wantCategory {
				t.Errorf("%s: category = %q, want %q", tc.file, blk.Frameworks.Category, tc.wantCategory)
			}
			if len(blk.Frameworks.Detection.StructuralMarkers) == 0 {
				t.Fatalf("%s: no structural_markers declared", tc.file)
			}

			// Happy path: a representative C++ source matches the framework.
			if marker, ok := anyMarkerMatches(blk, tc.happy); !ok {
				t.Errorf("%s: happy-path source matched no detection signature", tc.file)
			} else {
				t.Logf("%s: happy match on %q", tc.file, marker)
			}

			// Wrong-language no-op: the C/C++ signatures must not fire on a
			// same-purpose source written in an unrelated language.
			if marker, ok := anyMarkerMatches(blk, tc.wrongLang); ok {
				t.Errorf("%s: detection signature %q misfired on wrong-language source", tc.file, marker)
			}

			// No-match no-op: a plain C++ source using none of the framework
			// matches nothing.
			if marker, ok := anyMarkerMatches(blk, tc.noMatch); ok {
				t.Errorf("%s: detection signature %q misfired on no-match source", tc.file, marker)
			}
		})
	}
}

// TestCppGuiCvGameDetection_ScopedToCpp guards the wrong-language contract at
// the corpus level: each of the five frameworks' detection YAMLs is embedded
// only under rules/cpp/frameworks/ and not duplicated under another language
// bucket, so its C/C++ structural markers cannot be loaded as another
// language's rules.
func TestCppGuiCvGameDetection_ScopedToCpp(t *testing.T) {
	files := []string{"opencv.yaml", "dear_imgui.yaml", "cocos2d_x.yaml", "juce.yaml", "wxwidgets.yaml"}
	otherLangs := []string{"c", "go", "python", "rust", "java", "javascript_typescript", "csharp"}
	for _, f := range files {
		if _, err := rulesFS.ReadFile("rules/cpp/frameworks/" + f); err != nil {
			t.Errorf("%s: not embedded under rules/cpp/frameworks/: %v", f, err)
		}
		for _, lang := range otherLangs {
			if _, err := rulesFS.ReadFile("rules/" + lang + "/frameworks/" + f); err == nil {
				t.Errorf("%s: unexpectedly embedded under rules/%s/frameworks/ — should be cpp-only", f, lang)
			}
		}
	}
}
