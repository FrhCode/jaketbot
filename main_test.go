package main

import "testing"

func TestContainsKeywordCaseInsensitive(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		keyword string
		want    bool
	}{
		{name: "same case", body: "pendaftaran dibuka", keyword: "dibuka", want: true},
		{name: "different case", body: "Pendaftaran DIBUKA Hari Ini", keyword: "dibuka", want: true},
		{name: "absent", body: "pendaftaran ditutup", keyword: "dibuka", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsKeyword(tt.body, tt.keyword)
			if got != tt.want {
				t.Fatalf("containsKeyword(%q, %q) = %v, want %v", tt.body, tt.keyword, got, tt.want)
			}
		})
	}
}
