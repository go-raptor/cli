package generate

import "testing"

func TestCheckArgs(t *testing.T) {
	tests := []struct {
		typ  string
		args []string
		ok   bool
	}{
		{"resource", []string{"resource", "Course"}, true},
		{"resource", []string{"resource", "Course", "name:string", "division:ref"}, true},
		{"controller", []string{"controller", "Users"}, true},
		{"controller", []string{"controller", "Users", "extra"}, false},
	}
	for _, tt := range tests {
		if err := checkArgs(tt.typ, tt.args); (err == nil) != tt.ok {
			t.Errorf("checkArgs(%q, %v) = %v", tt.typ, tt.args, err)
		}
	}
}
