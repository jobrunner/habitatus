package classify

import (
	"testing"

	"github.com/jobrunner/habitatus/internal/rulepack"
)

// alwaysFalseLeaf and freeLeaf build minimal Leaf nodes for exercising the
// reach()/ruleReachable() tree walk in isolation, without going through a
// full rule pack. alwaysFalseLeaf models an "#NN Group" expression, which
// upstream forces to FALSE; freeLeaf models any ordinary expression, which
// can be either true or false depending on the plot.
func alwaysFalseLeaf() rulepack.Node {
	return rulepack.Leaf{Expr: rulepack.Expr{AlwaysFalse: true}}
}

func freeLeaf() rulepack.Node {
	return rulepack.Leaf{Expr: rulepack.Expr{AlwaysFalse: false}}
}

func TestRuleReachableOrWithOneAlwaysFalseBranchIsReachable(t *testing.T) {
	// "always-false OR free" is reachable: the free branch can be true.
	n := rulepack.Or{L: alwaysFalseLeaf(), R: freeLeaf()}
	if !ruleReachable(n) {
		t.Error("Or with one always-false branch and one free branch must be reachable")
	}
	// Symmetric in the other argument order.
	n = rulepack.Or{L: freeLeaf(), R: alwaysFalseLeaf()}
	if !ruleReachable(n) {
		t.Error("Or with one free branch and one always-false branch must be reachable")
	}
}

func TestRuleReachableOrWithBothAlwaysFalseIsUnreachable(t *testing.T) {
	n := rulepack.Or{L: alwaysFalseLeaf(), R: alwaysFalseLeaf()}
	if ruleReachable(n) {
		t.Error("Or with both branches always-false must be unreachable")
	}
}

func TestRuleReachableAndWithEitherAlwaysFalseOperandIsUnreachable(t *testing.T) {
	// "always-false AND free" is unreachable: the left side can never be
	// true, so the conjunction never can be either.
	n := rulepack.And{L: alwaysFalseLeaf(), R: freeLeaf()}
	if ruleReachable(n) {
		t.Error("And with an always-false left operand must be unreachable")
	}
	// Symmetric in the other argument order.
	n = rulepack.And{L: freeLeaf(), R: alwaysFalseLeaf()}
	if ruleReachable(n) {
		t.Error("And with an always-false right operand must be unreachable")
	}
}

func TestRuleReachableAndOfTwoFreeLeavesIsReachable(t *testing.T) {
	n := rulepack.And{L: freeLeaf(), R: freeLeaf()}
	if !ruleReachable(n) {
		t.Error("And of two free leaves must be reachable")
	}
}

func TestRuleReachableNotWithAlwaysFalseLeftOperandIsUnreachable(t *testing.T) {
	// Not is "L AND NOT R": if L can never be true, "L NOT R" never can
	// either, regardless of R.
	n := rulepack.Not{L: alwaysFalseLeaf(), R: freeLeaf()}
	if ruleReachable(n) {
		t.Error("Not with an always-false left operand must be unreachable")
	}
}

func TestRuleReachableNotWithFreeLeftAndAlwaysFalseRightIsReachable(t *testing.T) {
	// "free NOT always-false" is reachable: the always-false right side
	// trivially satisfies "R is false", and the free left side can be true.
	n := rulepack.Not{L: freeLeaf(), R: alwaysFalseLeaf()}
	if !ruleReachable(n) {
		t.Error("Not with a free left operand and an always-false right operand must be reachable")
	}
}
