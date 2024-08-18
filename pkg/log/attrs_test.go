package log

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_setAttrsAtPath(t *testing.T) {
	type args struct {
		parentAttrs []slog.Attr
		groups      []string
		attrs       []slog.Attr
	}
	tests := []struct {
		name string
		args args
		want []slog.Attr
	}{
		{
			name: "Flat",
			args: args{
				parentAttrs: nil,
				groups:      nil,
				attrs: []slog.Attr{
					slog.Int("a", 1),
				},
			},
			want: []slog.Attr{
				slog.Int("a", 1),
			},
		},
		{
			name: "MissingParent",
			args: args{
				parentAttrs: nil,
				groups:      []string{"a"},
				attrs: []slog.Attr{
					slog.Int("b", 2),
				},
			},
			want: []slog.Attr{
				slog.Group("a",
					slog.Int("b", 2)),
			},
		},
		{
			name: "MissingParents",
			args: args{
				parentAttrs: nil,
				groups:      []string{"a", "b"},
				attrs: []slog.Attr{
					slog.Int("c", 3),
				},
			},
			want: []slog.Attr{
				slog.Group("a",
					slog.Group("b",
						slog.Int("c", 3))),
			},
		},
		{
			name: "ExistingParents",
			args: args{
				parentAttrs: []slog.Attr{
					slog.Group("a",
						slog.Group("b",
							slog.Int("c", 3))),
				},
				groups: []string{"a", "b"},
				attrs: []slog.Attr{
					slog.Int("d", 4),
				},
			},
			want: []slog.Attr{
				slog.Group("a",
					slog.Group("b",
						slog.Int("c", 3),
						slog.Int("d", 4))),
			},
		},
		{
			name: "ExistingKey",
			args: args{
				parentAttrs: []slog.Attr{
					slog.Group("a",
						slog.Group("b",
							slog.Int("c", 3))),
				},
				groups: []string{"a", "b"},
				attrs: []slog.Attr{
					slog.Int("d", 4),
				},
			},
			want: []slog.Attr{
				slog.Group("a",
					slog.Group("b",
						slog.Int("c", 3),
						slog.Int("d", 4))),
			},
		},
		{
			name: "Merge",
			args: args{
				parentAttrs: []slog.Attr{
					slog.Group("a",
						slog.Group("b",
							slog.Int("c", 3))),
				},
				groups: []string{"a"},
				attrs: []slog.Attr{
					slog.Group("b",
						slog.Int("d", 4)),
					slog.Group("e",
						slog.Int("f", 6)),
				},
			},
			want: []slog.Attr{
				slog.Group("a",
					slog.Group("b",
						slog.Int("c", 3),
						slog.Int("d", 4)),
					slog.Group("e",
						slog.Int("f", 6))),
			},
		},
		{
			name: "MergeOverwrite",
			args: args{
				parentAttrs: []slog.Attr{
					slog.Group("a",
						slog.Group("b",
							slog.Int("c", 3))),
				},
				groups: []string{"a"},
				attrs: []slog.Attr{
					slog.Group("b",
						slog.Int("c", 1)),
				},
			},
			want: []slog.Attr{
				slog.Group("a",
					slog.Group("b",
						slog.Int("c", 1))),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SetAttrsAtPath(tt.args.parentAttrs, tt.args.groups, tt.args.attrs)
			assert.Equal(t, tt.want, got)
		})
	}
}
