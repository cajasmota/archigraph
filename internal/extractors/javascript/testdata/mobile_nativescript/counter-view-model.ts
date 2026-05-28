// NativeScript-Core view-model fixture (#2885) — modelled on a real
// `extends Observable` view-model from @nativescript/app-templates. Unlike the
// toy main-view-model.ts (whose branch is a bare-identifier compare the
// discriminator pass happens to catch), real NS view-models branch on MEMBER
// comparisons with relational operators, which the discriminator pass misses:
//
//   if (this._counter <= 0)     — member LHS, relational op, no literal match
//   if (this._x !== value)      — guard before notifyPropertyChange
//   this._mode ? ... : ...      — ternary on a member
//   switch (this._status)       — switch on a member
//
// Proves Data Flow/branch_conditions for NativeScript via the #2885 general
// branch-condition pass (Properties["branch_conditions"] + BRANCHES_ON edges).
import { Observable } from '@nativescript/core';

export class CounterViewModel extends Observable {
  private _counter = 0;
  private _status = 'idle';
  private _busy = false;

  // Real NS setter idiom: guard with a member !== comparison, then notify.
  // The guard `this._counter !== value` is a member comparison the
  // discriminator pass cannot see (LHS is not a bare identifier).
  set counter(value: number) {
    if (this._counter !== value) {
      this._counter = value;
      this.notifyPropertyChange('counter', value);
    }
  }

  get counter(): number {
    return this._counter;
  }

  // Relational comparison on a member — `<=` is outside the discriminator
  // operator set entirely.
  decrement() {
    if (this._counter <= 0) {
      return;
    }
    this.set('counter', this._counter - 1);
  }

  // Ternary on a member plus a switch on a member — neither is discriminator
  // shaped, both are real view-model branches.
  classify(): string {
    const label = this._busy ? 'working' : 'ready';
    switch (this._status) {
      case 'idle':
        return label;
      default:
        if (this._counter > 10) {
          return 'high';
        }
        return label;
    }
  }
}
