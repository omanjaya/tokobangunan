package domain

import (
	"errors"
	"testing"
)

func TestStatusOpname_IsValid(t *testing.T) {
	for _, s := range []StatusOpname{OpnameDraft, OpnameSelesai, OpnameApproved} {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if StatusOpname("foo").IsValid() {
		t.Error("foo should be invalid")
	}
	if StatusOpname("").IsValid() {
		t.Error("empty should be invalid")
	}
}

func TestStokOpname_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from StatusOpname
		to   StatusOpname
		want bool
	}{
		{OpnameDraft, OpnameSelesai, true},
		{OpnameSelesai, OpnameApproved, true},
		{OpnameDraft, OpnameApproved, false},
		{OpnameSelesai, OpnameDraft, false},
		{OpnameApproved, OpnameDraft, false},
		{OpnameApproved, OpnameSelesai, false},
		{OpnameDraft, OpnameDraft, false},
		{OpnameSelesai, OpnameSelesai, false},
		{OpnameApproved, OpnameApproved, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			if got != tt.want {
				t.Errorf("CanTransitionTo(%s->%s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestStokOpname_Validate(t *testing.T) {
	base := func() *StokOpname {
		return &StokOpname{GudangID: 1, Status: OpnameDraft}
	}
	tests := []struct {
		name    string
		mutate  func(o *StokOpname)
		wantErr error
	}{
		{"ok", func(o *StokOpname) {}, nil},
		{"gudang nol", func(o *StokOpname) { o.GudangID = 0 }, ErrOpnameGudangWajib},
		{"gudang negatif", func(o *StokOpname) { o.GudangID = -1 }, ErrOpnameGudangWajib},
		{"status invalid", func(o *StokOpname) { o.Status = StatusOpname("xx") }, ErrOpnameStatusInvalid},
		{"status kosong", func(o *StokOpname) { o.Status = "" }, ErrOpnameStatusInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := base()
			tt.mutate(o)
			err := o.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
