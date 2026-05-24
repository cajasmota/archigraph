package solidity_test

// Issue #2114 — same self-recursion guard bug class as the Java fix.
// The Solidity guard was:
//
//	callerLeaf := callerName
//	if dot := strings.LastIndexByte(callerName, '.'); dot >= 0 {
//	    callerLeaf = callerName[dot+1:]
//	}
//	if leaf == callerLeaf { return }   // BUG: fires on cross-contract dotted calls
//
// When client-fixture-A.ERC20Vault.transfer() called
// token.transfer(to, amount), the resolved target "token.transfer" had its
// leaf "transfer" matched against callerLeaf("ERC20Vault.transfer") == "transfer"
// → edge silently dropped as "self-recursion".
//
// Fix: restrict the self-recursion skip to bare-name (undotted) targets only.
// A dotted target like "token.transfer" is a cross-contract call and must
// never be filtered by a bare-leaf match against the caller's own leaf name.

import (
	"testing"
)

// TestSolidity_CallsFieldReceiverDottedTarget_SameLeaf (#2114): a contract
// function named "transfer" that delegates to a field receiver
// "token.transfer(to, amount)" MUST emit a CALLS edge to "token.transfer".
// Before the fix, leaf("token.transfer") == "transfer" matched
// callerLeaf("ERC20Vault.transfer") == "transfer" and the edge was dropped.
func TestSolidity_CallsFieldReceiverDottedTarget_SameLeaf(t *testing.T) {
	src := `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract ERC20Vault {
    function transfer(address to, uint256 amount) external {
        token.transfer(to, amount);
    }
}
`
	ents := runSolidity(t, src, "ERC20Vault.sol")
	if !solHasRel(ents, "ERC20Vault.transfer", "SCOPE.Operation", "CALLS", "token.transfer") {
		t.Errorf("ERC20Vault.transfer has no CALLS edge to token.transfer; want cross-contract call preserved")
	}
}

// TestSolidity_SelfRecursionStillDropped (#2114): true self-recursion (a bare
// function call to itself without a receiver) MUST still be suppressed.
func TestSolidity_SelfRecursionStillDropped(t *testing.T) {
	src := `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Looper {
    function loop() external {
        loop();
    }
}
`
	ents := runSolidity(t, src, "Looper.sol")
	for _, ent := range ents {
		if ent.Name == "Looper.loop" {
			for _, r := range ent.Relationships {
				if r.Kind == "CALLS" && r.ToID == "loop" {
					t.Errorf("self-recursion should not produce a CALLS edge: %+v", r)
				}
			}
			return
		}
	}
	// If we didn't find the entity that's also a test failure
	t.Fatal("expected entity 'Looper.loop' (SCOPE.Operation)")
}
