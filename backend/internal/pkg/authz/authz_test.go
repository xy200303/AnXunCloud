package authz

import "testing"

// 回归测试：keyMatch2 曾把 report:sign:inspector 当作 report:*:* 通配，
// 导致只有「巡检员确认」权限的用户通过「代签」校验（权限点同前缀串权）。
func newTestEnforcer(t *testing.T, policies, groupings [][]string) interface {
	Enforce(rvals ...interface{}) (bool, error)
} {
	t.Helper()
	e, err := newEnforcer(nil)
	if err != nil {
		t.Fatalf("newEnforcer: %v", err)
	}
	if len(policies) > 0 {
		if _, err := e.AddPolicies(policies); err != nil {
			t.Fatalf("AddPolicies: %v", err)
		}
	}
	if len(groupings) > 0 {
		if _, err := e.AddGroupingPolicies(groupings); err != nil {
			t.Fatalf("AddGroupingPolicies: %v", err)
		}
	}
	return e
}

func TestPermMatchNoCrossPrefixBleed(t *testing.T) {
	e := newTestEnforcer(t,
		[][]string{{"role:inspector", DefaultDomain, "report:sign:inspector"}},
		[][]string{{"user:u1", "role:inspector", DefaultDomain}},
	)
	cases := []struct {
		obj  string
		want bool
	}{
		{"report:sign:inspector", true},  // 持有的权限点等值命中
		{"report:sign:proxy", false}, // 同前缀不同权限点不得通过（本次 bug）
		{"report:sign", false},
		{"report:list", false},
		{"report:generate", false},
	}
	for _, c := range cases {
		ok, err := e.Enforce("user:u1", DefaultDomain, c.obj)
		if err != nil {
			t.Fatalf("Enforce %s: %v", c.obj, err)
		}
		if ok != c.want {
			t.Errorf("Enforce(report:sign:inspector 持有, %s) = %v, want %v", c.obj, ok, c.want)
		}
	}
}

func TestPermMatchModuleWildcard(t *testing.T) {
	e := newTestEnforcer(t,
		[][]string{{"role:manager", DefaultDomain, "system:user:*"}},
		[][]string{{"user:u2", "role:manager", DefaultDomain}},
	)
	cases := []struct {
		obj  string
		want bool
	}{
		{"system:user:list", true},
		{"system:user:create", true},
		{"system:user:", true}, // 前缀边界
		{"system:role:list", false},
		{"system:user", false}, // 不带尾冒号不命中
		{"inspection:point:list", false},
	}
	for _, c := range cases {
		ok, err := e.Enforce("user:u2", DefaultDomain, c.obj)
		if err != nil {
			t.Fatalf("Enforce %s: %v", c.obj, err)
		}
		if ok != c.want {
			t.Errorf("Enforce(system:user:* 持有, %s) = %v, want %v", c.obj, ok, c.want)
		}
	}
}

func TestPermMatchSuperWildcard(t *testing.T) {
	e := newTestEnforcer(t,
		[][]string{{"role:super_admin", DefaultDomain, "*"}},
		[][]string{{"user:u3", "role:super_admin", DefaultDomain}},
	)
	for _, obj := range []string{"system:user:delete", "report:sign:proxy", "anything:at:all"} {
		ok, err := e.Enforce("user:u3", DefaultDomain, obj)
		if err != nil || !ok {
			t.Errorf("超管通配 Enforce(%s) = %v, err=%v, want true", obj, ok, err)
		}
	}
}

func TestPermMatchExactNoWildcardExpansion(t *testing.T) {
	// 等值策略不衍生前缀权限：system:user:list 不得推出 system:user:delete
	e := newTestEnforcer(t,
		[][]string{{"role:r", DefaultDomain, "system:user:list"}},
		[][]string{{"user:u4", "role:r", DefaultDomain}},
	)
	for _, c := range []struct {
		obj  string
		want bool
	}{
		{"system:user:list", true},
		{"system:user:delete", false},
		{"system:user", false},
	} {
		ok, _ := e.Enforce("user:u4", DefaultDomain, c.obj)
		if ok != c.want {
			t.Errorf("Enforce(system:user:list 持有, %s) = %v, want %v", c.obj, ok, c.want)
		}
	}
}
