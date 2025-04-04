package gopackages

import "testing"

func TestParser_allowedByTags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		s    string
		want bool
	}{
		{
			name: "true",
			tags: []string{"foo"},
			s:    "foo",
			want: true,
		},
		{
			name: "false",
			tags: []string{"bar"},
			s:    "foo",
			want: false,
		},
		{
			name: "and",
			tags: []string{"bar"},
			s:    "foo && bar",
			want: false,
		},
		{
			name: "or",
			tags: []string{"bar"},
			s:    "bar || foo",
			want: true,
		},
		{

			name: "not",
			tags: []string{"bar"},
			s:    "bar && !foo",
			want: true,
		},
		{

			name: "parens",
			tags: []string{"bar", "foo"},
			s:    "(!foo || !spam) && foo",
			want: true,
		},
		{
			name: "outer not",
			tags: []string{"js", "wasm"},
			s:    "!(js && wasm)",
			want: false,
		},
		{
			name: "precedence",
			tags: []string{"true"},
			s:    "true && true || false && false",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Parser{
				env: Env{
					Tags: tt.tags,
				},
			}

			got := p.allowedByTags(tt.s)

			if got != tt.want {
				t.Fail()
			}
		})
	}
}
