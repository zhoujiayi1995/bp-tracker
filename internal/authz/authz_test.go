package authz

import "testing"

func TestAccessMatrix(t *testing.T) {
	const (
		alice  = 1 // owns self profile, manages grandma, in family 10
		bob    = 2 // same family as alice
		carol  = 3 // different family
		dave   = 4 // no family
		family = 10
		other  = 20
	)
	aliceSelf := Target{UserID: alice, OwnerFamilyID: family}
	grandma := Target{ManagerUserID: alice, OwnerFamilyID: family}
	aliceSelfAfterLeave := Target{UserID: alice}

	cases := []struct {
		name      string
		subject   Subject
		target    Target
		wantRead  bool
		wantWrite bool
	}{
		{"owner on self profile", Subject{alice, family}, aliceSelf, true, true},
		{"manager on managed profile", Subject{alice, family}, grandma, true, true},
		{"family member on self profile", Subject{bob, family}, aliceSelf, true, false},
		{"family member on managed profile", Subject{bob, family}, grandma, true, false},
		{"other family", Subject{carol, other}, aliceSelf, false, false},
		{"no family", Subject{dave, 0}, grandma, false, false},
		{"former family member after owner left", Subject{bob, family}, aliceSelfAfterLeave, false, false},
		{"owner after leaving keeps access", Subject{alice, 0}, aliceSelfAfterLeave, true, true},
		{"manager after leaving keeps managed profile", Subject{alice, 0}, Target{ManagerUserID: alice}, true, true},
		{"two users without family are not related", Subject{dave, 0}, Target{UserID: carol}, false, false},
		{"anonymous subject", Subject{}, Target{}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanRead(tc.subject, tc.target); got != tc.wantRead {
				t.Errorf("CanRead = %v, want %v", got, tc.wantRead)
			}
			if got := CanWrite(tc.subject, tc.target); got != tc.wantWrite {
				t.Errorf("CanWrite = %v, want %v", got, tc.wantWrite)
			}
		})
	}
}
