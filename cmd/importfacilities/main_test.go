package main

import (
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// ★ 주소에서 시군구를 잘못 읽으면 관할 판정이 통째로 틀린다.
// 관악구 센터가 강남구 사람에게 안내되면 그건 안내가 아니라 헛걸음이다.
func TestSplitRegion(t *testing.T) {
	tests := []struct {
		name         string
		road, lot    string
		fallbackSido string
		wantSido     string
		wantSigungu  string
	}{
		{
			name:        "도로명주소에서 시도·시군구",
			road:        "서울특별시 관악구 관악로 145, 3동 4층",
			wantSido:    "서울특별시",
			wantSigungu: "관악구",
		},
		{
			name:        "광역시",
			road:        "부산광역시 해운대구 센텀중앙로 1",
			wantSido:    "부산광역시",
			wantSigungu: "해운대구",
		},
		{
			name:        "도 + 시",
			road:        "경기도 성남시 분당구 판교로 1",
			wantSido:    "경기도",
			wantSigungu: "성남시",
		},
		{
			name:        "특별자치도",
			road:        "제주특별자치도 제주시 중앙로 1",
			wantSido:    "제주특별자치도",
			wantSigungu: "제주시",
		},
		{
			name:        "군 단위",
			road:        "강원특별자치도 양양군 양양읍 1",
			wantSido:    "강원특별자치도",
			wantSigungu: "양양군",
		},
		{
			name:        "도로명이 없으면 지번주소를 쓴다",
			road:        "",
			lot:         "서울특별시 종로구 사직동 1-1",
			wantSido:    "서울특별시",
			wantSigungu: "종로구",
		},
		{
			name:         "★ 시도가 빠진 주소는 -sido 로 보완한다",
			road:         "관악구 봉천로 100",
			fallbackSido: "서울특별시",
			wantSido:     "서울특별시",
			wantSigungu:  "관악구",
		},
		{
			name:        "★ 읽을 수 없으면 비운다 — 추측하지 않는다",
			road:        "주소미상",
			wantSido:    "",
			wantSigungu: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sido, sigungu := splitRegion(tt.road, tt.lot, tt.fallbackSido)
			if sido != tt.wantSido {
				t.Errorf("sido = %q, want %q", sido, tt.wantSido)
			}
			if sigungu != tt.wantSigungu {
				t.Errorf("sigungu = %q, want %q", sigungu, tt.wantSigungu)
			}
		})
	}
}

func TestGuessType(t *testing.T) {
	tests := []struct {
		raw  string
		want model.FacilityType
	}{
		{"지역아동센터", model.FacilityChildCenter},
		{"지역아동센터(일반)", model.FacilityChildCenter},
		{"종합사회복지관", model.FacilityCommunityWelfare},
		{"노인복지관", model.FacilityElderly},
		{"장애인복지관", model.FacilityDisability},
		{"정신건강복지센터", model.FacilityMentalHealth},
		{"지역자활센터", model.FacilitySelfSufficiency},
		{"한부모가족복지시설", model.FacilitySingleParent},
		{"건강가정지원센터", model.FacilityFamilyCenter},
		// ★ 모르는 값은 OTHER 다. 틀린 분류로 넣는 것보다 낫다
		{"무슨무슨시설", model.FacilityOther},
		{"", model.FacilityOther},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			if got := guessType(tt.raw); got != tt.want {
				t.Errorf("guessType(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// ★ 없는 번호를 만들어내지 않는다. 이 테스트가 그것을 지킨다.
func TestNormalizePhone(t *testing.T) {
	tests := []struct{ in, want string }{
		{"02-880-1234", "02-880-1234"},
		{"  02-880-1234  ", "02-880-1234"},
		{"02-880-1234 (내선 3)", "02-880-1234"},
		{"02-880-1234, 02-880-5678", "02-880-1234"},
		{"129", "129"},
		// 숫자가 없으면 전화번호가 아니다
		{"없음", ""},
		{"-", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizePhone(tt.in); got != tt.want {
				t.Errorf("normalizePhone(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://example.kr", "https://example.kr"},
		{"http://example.kr", "http://example.kr"},
		{"www.example.kr", "https://www.example.kr"},
		{"example.or.kr", "https://example.or.kr"},
		{"-", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeURL(tt.in); got != tt.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ★ 연락할 수 없는 시설은 아예 만들지 않는다.
func TestConvertSkipsUnreachable(t *testing.T) {
	col := newColumns([]string{"시설명", "소재지도로명주소", "전화번호", "홈페이지주소", "데이터기준일자"})

	rows := [][]string{
		{"연락되는곳", "서울특별시 관악구 관악로 1", "02-111-1111", "", "2026-08-31"},
		{"연락안되는곳", "서울특별시 관악구 관악로 2", "", "", "2026-08-31"},
		{"홈페이지만있는곳", "서울특별시 관악구 관악로 3", "", "example.kr", "2026-08-31"},
	}

	got, skipped := convert(rows, col, model.FacilityChildCenter, "서울특별시", "seoul")

	if len(got) != 2 {
		t.Fatalf("변환 %d건, want 2건 (연락 불가 1건은 빠져야 한다)", len(got))
	}
	if len(skipped) != 1 {
		t.Errorf("건너뜀 %d건, want 1건: %v", len(skipped), skipped)
	}
	for _, f := range got {
		if f.Name == "연락안되는곳" {
			t.Error("★ 연락할 수 없는 시설이 들어갔다")
		}
		if f.Coverage.Sigungu != "관악구" {
			t.Errorf("%s 의 관할 = %q, want 관악구", f.Name, f.Coverage.Sigungu)
		}
	}
}
