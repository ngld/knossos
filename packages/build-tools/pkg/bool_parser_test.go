package pkg

import "testing"

func TestAnd(t *testing.T) {
	expr, err := ParseBoolExpr("a && b")
	if err != nil {
		t.Fatalf("parser failed with %s for \"a && b\"", err)
	}

	vars := map[string]bool{"a": true, "b": true}
	if !expr.Eval(vars) {
		t.Fatal("a=true, b=true failed")
	}

	vars["a"] = false
	if expr.Eval(vars) {
		t.Fatal("a=false, b=true should've failed but didn't")
	}

	vars["a"] = true
	vars["b"] = false
	if expr.Eval(vars) {
		t.Fatal("a=true, b=false should've failed but didn't")
	}

	vars["a"] = false
	if expr.Eval(vars) {
		t.Fatal("a=false, b=false should've failed but didn't")
	}
}

func TestOr(t *testing.T) {
	expr, err := ParseBoolExpr("a || b")
	if err != nil {
		t.Fatalf("parser failed with %s for \"a || b\"", err)
	}

	vars := map[string]bool{"a": true, "b": true}
	if !expr.Eval(vars) {
		t.Fatal("a=true, b=true failed")
	}

	vars["a"] = false
	if !expr.Eval(vars) {
		t.Fatal("a=false, b=true failed")
	}

	vars["a"] = true
	vars["b"] = false
	if !expr.Eval(vars) {
		t.Fatal("a=true, b=false failed")
	}

	vars["a"] = false
	if expr.Eval(vars) {
		t.Fatal("a=false, b=false should've failed but didn't")
	}
}

func TestParens(t *testing.T) {
	exprs := []string{"(a && b) || c", "c || (a && b)"}
	for _, expr := range exprs {
		expr, err := ParseBoolExpr(expr)
		if err != nil {
			t.Fatalf("parser failed with %s for \"(a && b) || c\"", err)
		}

		vars := map[string]bool{"a": true, "b": true, "c": false}
		if !expr.Eval(vars) {
			t.Fatal("a=true, b=true, c=false failed")
		}

		vars["a"] = false
		if expr.Eval(vars) {
			t.Fatal("a=false, b=true, c=false should've failed but didn't")
		}

		vars["a"] = true
		vars["b"] = false
		if expr.Eval(vars) {
			t.Fatal("a=true, b=false, c=false should've failed but didn't")
		}

		vars["a"] = false
		if expr.Eval(vars) {
			t.Fatal("a=false, b=false, c=false should've failed but didn't")
		}

		vars["c"] = true
		if !expr.Eval(vars) {
			t.Fatal("a=false, b=false, c=true failed")
		}
	}
}

func TestInvalidStrings(t *testing.T) {
	_, err := ParseBoolExpr("var+fds")
	if err == nil {
		t.Fatal("var+fds didn't fail")
	}

	if err.Error() != "expected a-z, A-Z or _ but found +" {
		t.Fatalf("var+fds failed with the wrong error: %s", err.Error())
	}

	_, err = ParseBoolExpr("a && ;f")
	if err == nil {
		t.Fatal("a && ;f didn't fail")
	}

	if err.Error() != "expected a-z, A-Z or _ but found ;" {
		t.Fatalf("a && ;f failed with the wrong error: %s", err.Error())
	}

	_, err = ParseBoolExpr("a &| b")
	if err == nil {
		t.Fatal("a &| didn't fail")
	}

	if err.Error() != "expected & but found |" {
		t.Fatalf("a &| failed with the wrong error: %s", err.Error())
	}

	_, err = ParseBoolExpr("a ++ b")
	if err == nil {
		t.Fatal("a ++ b didn't fail")
	}

	if err.Error() != "expected operator (&& or ||) but found +" {
		t.Fatalf("a ++ b failed with the wrong error: %s", err.Error())
	}
}

func TestChains(t *testing.T) {
	exprs := []string{"a && b && c", "a || b && c", "a && (b || c) && d", "(a && !b) || (c && d)", "darwin && (amd64 || !arm64) && ci"}
	for _, expr := range exprs {
		_, err := ParseBoolExpr(expr)
		if err != nil {
			t.Fatalf("%s failed with %s", expr, err)
		}
	}
}

func TestNegate(t *testing.T) {
	expr, err := ParseBoolExpr("!a")
	if err != nil {
		t.Fatalf("!a failed with %s", err)
	}

	vars := map[string]bool{"a": true}
	if expr.Eval(vars) {
		t.Fatal("a=true didn't fail but should've")
	}

	vars["a"] = false
	if !expr.Eval(vars) {
		t.Fatal("a=false failed")
	}
}
