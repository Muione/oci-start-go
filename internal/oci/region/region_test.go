package region

import "testing"

func TestCodeByName(t *testing.T) {
	cases := map[string]string{
		"东京":         "ap-tokyo-1",
		"亚太-日本东部东京": "ap-tokyo-1",
		"阿什本":        "us-ashburn-1",
		"ap-tokyo-1": "ap-tokyo-1", // already a code, no chinese
		"未知地方":      "未知地方", // unresolved
		"":          "",
	}
	for in, want := range cases {
		if got := CodeByName(in); got != want {
			t.Errorf("CodeByName(%q)=%q want %q", in, got, want)
		}
	}
	if n := NameByCode("ap-tokyo-1"); n != "东京" {
		t.Errorf("NameByCode=%q want 东京", n)
	}
	if all := All(); len(all) != 45 {
		t.Errorf("All len=%d want 45", len(all))
	}
}
