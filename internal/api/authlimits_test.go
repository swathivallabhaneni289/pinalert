package api

import (
	"testing"
	"time"
)

// TestAuthLimitsResolve pins resolveAuthLimits: a zero Deps value gives the
// defaults for every field, and a field a caller set is kept (the
// resolveRateLimits contract, for the code and password buckets).
func TestAuthLimitsResolve(t *testing.T) {
	if got := resolveAuthLimits(AuthLimits{}); got != AuthLimitsDefault {
		t.Fatalf("zero AuthLimits resolved to %+v, want %+v", got, AuthLimitsDefault)
	}

	custom := BucketLimit{Every: 7 * time.Second, Burst: 3}
	got := resolveAuthLimits(AuthLimits{SignInDevice: custom, UsernameSave: custom})
	if got.SignInDevice != custom || got.UsernameSave != custom {
		t.Fatalf("set fields were replaced: SignInDevice=%+v UsernameSave=%+v, want %+v", got.SignInDevice, got.UsernameSave, custom)
	}
	if got.SignInIP != AuthLimitsDefault.SignInIP || got.CodeSendDevice != AuthLimitsDefault.CodeSendDevice ||
		got.CodeSendIP != AuthLimitsDefault.CodeSendIP || got.CodeSendNoCookie != AuthLimitsDefault.CodeSendNoCookie || got.SignInNoCookie != AuthLimitsDefault.SignInNoCookie ||
		got.CodeTryDevice != AuthLimitsDefault.CodeTryDevice || got.UsernameCheck != AuthLimitsDefault.UsernameCheck {
		t.Fatalf("unset fields were not defaulted: %+v", got)
	}

	for name, l := range map[string]BucketLimit{
		"SignInDevice": got.SignInDevice, "SignInIP": got.SignInIP,
		"CodeSendDevice": got.CodeSendDevice, "CodeSendIP": got.CodeSendIP,
		"CodeSendNoCookie": got.CodeSendNoCookie, "CodeTryDevice": got.CodeTryDevice,
		"SignInNoCookie": got.SignInNoCookie,
		"UsernameCheck":  got.UsernameCheck, "UsernameSave": got.UsernameSave,
	} {
		if l.Every <= 0 || l.Burst < 1 {
			t.Errorf("%s resolved to an invalid bucket %+v", name, l)
		}
	}
}
