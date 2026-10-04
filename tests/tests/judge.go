package tests

import (
	"encoding/json"
	"fmt"
	"net/url"
	"tests/util"
)

// --- Judge CRUD Tests ---

// AddAndDeleteJudge verifies a judge can be created and then deleted, disappearing from the list
func AddAndDeleteJudge(context *util.Context) util.Result {
	// Add a unique judge
	name := "Delete Test Judge"
	addRes := util.PostRequest(context.Logger, "/judge/new", util.H{
		"name":  "Delete Test Judge",
		"track": "",
		"notes": "",
	}, util.AdminAuth())
	if !util.IsOk(addRes) {
		return util.NewResult(false, "Failed to add judge: "+addRes)
	}

	// Find their ID
	id, result := findJudgeIDByName(context, name)
	if !result.Success {
		return result
	}

	// Delete the judge
	delRes := util.DeleteRequest(context.Logger, "/judge/"+id, util.AdminAuth())
	if !util.IsOk(delRes) {
		return util.NewResult(false, "Failed to delete judge: "+delRes)
	}

	// Confirm they no longer appear in the list
	_, result = findJudgeIDByName(context, name)
	if result.Success {
		return util.NewResult(false, "Judge still appears in list after deletion")
	}

	return util.ResultOk()
}

// EditJudge verifies that judge info can be updated and the changes persist
func EditJudge(context *util.Context) util.Result {
	name := "Edit Test Judge"
	addRes := util.PostRequest(context.Logger, "/judge/new", util.H{
		"name":  "Edit Test Judge",
		"track": "",
		"notes": "original notes",
	}, util.AdminAuth())
	if !util.IsOk(addRes) {
		return util.NewResult(false, "Failed to add judge for edit test: "+addRes)
	}

	id, result := findJudgeIDByName(context, name)
	if !result.Success {
		return result
	}

	// Edit the judge
	editRes := util.PutRequest(context.Logger, "/judge/"+id, util.H{
		"name":  "Edited Judge Name",
		"notes": "updated notes",
	}, util.AdminAuth())
	if !util.IsOk(editRes) {
		return util.NewResult(false, "Failed to edit judge: "+editRes)
	}

	// Verify the change
	listRes := util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())
	updatedName := findJudgeFieldByName(listRes, "Edited Judge Name", "name")
	if updatedName != "Edited Judge Name" {
		return util.NewResult(false, fmt.Sprintf("Judge name not updated: expected 'Edited Judge Name', got '%s'", updatedName))
	}

	return util.ResultOk()
}

// JudgeCodeLoginDisabled checks that QR registration is the only judge entry API.
func JudgeCodeLoginDisabled(context *util.Context) util.Result {
	status, _ := util.PostRequestWithStatus(context.Logger, "/judge/login", util.H{"code": "12345678"}, util.DefaultAuth())
	if status != 410 {
		return util.NewResult(false, "Removed judge code login should return 410")
	}
	return util.ResultOk()
}

// JudgeWelcomeFlow verifies the read_welcome flag can be set and queried
func JudgeWelcomeFlow(context *util.Context) util.Result {
	token, result := createNamedJudge(context, "Welcome Test Judge")
	if !result.Success {
		return result
	}

	auth := util.JudgeAuth(token)

	// Initially read_welcome should be false
	getRes := util.GetRequest(context.Logger, "/judge/welcome", auth)
	if util.IsOk(getRes) {
		return util.NewResult(false, "read_welcome should initially be false (not ok)")
	}

	// Mark welcome as read
	putRes := util.PostRequest(context.Logger, "/judge/welcome", util.H{}, auth)
	if !util.IsOk(putRes) {
		return util.NewResult(false, "Failed to set read_welcome: "+putRes)
	}

	// Now it should return ok
	getRes2 := util.GetRequest(context.Logger, "/judge/welcome", auth)
	if !util.IsOk(getRes2) {
		return util.NewResult(false, "read_welcome should be true after PUT /judge/welcome")
	}

	return util.ResultOk()
}

// HideJudge verifies that hiding a judge marks them as inactive
func HideJudge(context *util.Context) util.Result {
	name := "Hide Test Judge"
	addRes := util.PostRequest(context.Logger, "/judge/new", util.H{
		"name":  "Hide Test Judge",
		"track": "",
		"notes": "",
	}, util.AdminAuth())
	if !util.IsOk(addRes) {
		return util.NewResult(false, "Failed to add judge: "+addRes)
	}

	id, result := findJudgeIDByName(context, name)
	if !result.Success {
		return result
	}

	hideRes := util.PutRequest(context.Logger, "/judge/hide/"+id, util.H{"hide": true}, util.AdminAuth())
	if !util.IsOk(hideRes) {
		return util.NewResult(false, "Failed to hide judge: "+hideRes)
	}

	// Confirm active is now false
	listRes := util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())
	active := findJudgeFieldByName(listRes, name, "active")
	if active != "false" {
		return util.NewResult(false, fmt.Sprintf("Judge should be inactive after hiding, got active=%s", active))
	}

	return util.ResultOk()
}

// JudgeStatsReflectAdditions checks that judge stats update after adding judges
func JudgeStatsReflectAdditions(context *util.Context) util.Result {
	// Get current count
	statsBefore := util.GetRequest(context.Logger, "/judge/stats", util.AdminAuth())
	numBefore := util.ExtractInt(statsBefore, "num")

	// Add a judge
	addRes := util.PostRequest(context.Logger, "/judge/new", util.H{
		"name":  "Stats Test Judge",
		"track": "",
		"notes": "",
	}, util.AdminAuth())
	if !util.IsOk(addRes) {
		return util.NewResult(false, "Failed to add judge: "+addRes)
	}

	statsAfter := util.GetRequest(context.Logger, "/judge/stats", util.AdminAuth())
	numAfter := util.ExtractInt(statsAfter, "num")

	if numAfter != numBefore+1 {
		return util.NewResult(false, fmt.Sprintf("Judge count should increase by 1: before=%d, after=%d", numBefore, numAfter))
	}

	return util.ResultOk()
}

// --- Helpers ---

// createNamedJudge registers a general judge through QR and returns their session token.
func createNamedJudge(context *util.Context, name string) (string, util.Result) {
	return createTrackJudge(context, name, "")
}

// createTrackJudge registers a judge using the assigned judging pool's QR code.
func createTrackJudge(context *util.Context, name string, track string) (string, util.Result) {
	path := "/admin/qr"
	if track != "" {
		path += "/" + url.PathEscape(track)
	}
	codeRes := util.PostRequest(context.Logger, path, nil, util.AdminAuth())
	code := util.ExtractString(codeRes, "qr_code")
	if code == "" {
		return "", util.NewResult(false, "Could not generate QR registration code")
	}
	res := util.PostRequest(context.Logger, "/qr/add", util.H{"name": name, "track": track, "code": code}, util.DefaultAuth())
	token := util.ExtractString(res, "token")
	if token == "" {
		return "", util.NewResult(false, "QR registration did not return a token")
	}
	status, body := util.GetRequestWithStatus(context.Logger, "/judge", util.JudgeAuth(token))
	if status != 200 || util.ExtractString(body, "name") != name || util.ExtractString(body, "track") != track {
		return "", util.NewResult(false, "QR session did not authenticate the correct judge and track")
	}
	return token, util.ResultOk()
}

// findJudgeIDByName scans the judge list for a judge with the given name and returns their ID
func findJudgeIDByName(context *util.Context, name string) (string, util.Result) {
	listRes := util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())
	id := findJudgeFieldByName(listRes, name, "id")
	if id == "" {
		return "", util.NewResult(false, "Could not find judge with name '"+name+"' in judge list")
	}
	return id, util.ResultOk()
}

// findJudgeFieldByName scans a judge list JSON body and returns the value of 'field' for the judge with the given name
func findJudgeFieldByName(body string, name string, field string) string {
	var judges []map[string]any
	if err := json.Unmarshal([]byte(body), &judges); err != nil {
		return ""
	}
	for _, judge := range judges {
		if judge["name"] == name {
			if val, ok := judge[field]; ok {
				return fmt.Sprintf("%v", val)
			}
		}
	}
	return ""
}

// QRCheckEmptyCodeRejected verifies that /qr/check rejects an empty code.
// Before the fix, options.QRCode defaulted to "" so submitting "" satisfied
// the equality check on a fresh instance with no QR generated yet.
func QRCheckEmptyCodeRejected(context *util.Context) util.Result {
	res := util.PostRequest(context.Logger, "/qr/check", util.H{"code": ""}, util.DefaultAuth())
	return util.AssertNotOk(res, "Empty string should not pass QR code check")
}

// QRCheckTrackEmptyCodeRejected verifies that /qr/check/:track rejects an
// empty code. Track QR codes also default to "" and were equally bypassable.
func QRCheckTrackEmptyCodeRejected(context *util.Context) util.Result {
	res := util.PostRequest(context.Logger, "/qr/check/unset-track", util.H{"code": ""}, util.DefaultAuth())
	return util.AssertNotOk(res, "Empty string should not pass track QR code check")
}

// QRAddEmptyCodeDoesNotCreateJudge verifies that /qr/add rejects an empty
// code and does NOT create a judge. Checks judge count before and after to
// catch a silently-created judge even if the response body looks benign.
func QRAddEmptyCodeDoesNotCreateJudge(context *util.Context) util.Result {
	listBefore := util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())
	countBefore := countJudgesInList(listBefore)

	res := util.PostRequest(context.Logger, "/qr/add", util.H{
		"name":  "Attacker",
		"notes": "",
		"code":  "",
	}, util.DefaultAuth())

	if util.IsOk(res) {
		return util.NewResult(false, "POST /qr/add with empty code should be rejected")
	}

	listAfter := util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())
	countAfter := countJudgesInList(listAfter)
	if countAfter > countBefore {
		return util.NewResult(false, "POST /qr/add with empty code must not create a judge — judge count increased")
	}

	return util.ResultOk()
}

// QRAddGarbageCodeDoesNotCreateJudge verifies that a non-empty but invalid
// code is also rejected — guards against a fix that only special-cases "".
func QRAddGarbageCodeDoesNotCreateJudge(context *util.Context) util.Result {
	listBefore := util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())
	countBefore := countJudgesInList(listBefore)

	res := util.PostRequest(context.Logger, "/qr/add", util.H{
		"name":  "Attacker",
		"notes": "",
		"code":  "THIS-IS-NOT-A-REAL-QR-CODE",
	}, util.DefaultAuth())

	if util.IsOk(res) {
		return util.NewResult(false, "POST /qr/add with invalid code should be rejected")
	}

	listAfter := util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())
	countAfter := countJudgesInList(listAfter)
	if countAfter > countBefore {
		return util.NewResult(false, "POST /qr/add with invalid code must not create a judge — judge count increased")
	}

	return util.ResultOk()
}

// QRValidFlowStillWorks verifies the legitimate QR signup flow still works
// after the fix: generate a code as admin, verify it, then use it to add a judge.
func QRValidFlowStillWorks(context *util.Context) util.Result {
	// POST /admin/qr returns {"qr_code":"..."} directly, not {"ok":1}
	genRes := util.PostRequest(context.Logger, "/admin/qr", nil, util.AdminAuth())
	qrCode := util.ExtractString(genRes, "qr_code")
	if qrCode == "" {
		return util.NewResult(false, "POST /admin/qr did not return a qr_code: "+genRes)
	}

	checkRes := util.PostRequest(context.Logger, "/qr/check", util.H{"code": qrCode}, util.DefaultAuth())
	if !util.IsOk(checkRes) {
		return util.NewResult(false, "Valid QR code should pass /qr/check: "+checkRes)
	}

	countBefore := countJudgesInList(util.GetRequest(context.Logger, "/judge/list", util.AdminAuth()))
	for _, body := range []util.H{
		{"name": "   ", "code": qrCode},
		{"name": "Invalid Track Judge", "track": "missing-track", "code": qrCode},
	} {
		status, _ := util.PostRequestWithStatus(context.Logger, "/qr/add", body, util.DefaultAuth())
		if status != 400 {
			return util.NewResult(false, "Invalid QR registration should return 400")
		}
	}
	if countJudgesInList(util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())) != countBefore {
		return util.NewResult(false, "Invalid registration created a judge")
	}
	var tokens []string
	for _, name := range []string{"Legitimate QR Judge", "Second QR Judge"} {
		status, body := util.PostRequestWithStatus(context.Logger, "/qr/add", util.H{
			"name": "  " + name + "  ", "code": qrCode,
		}, util.DefaultAuth())
		token := util.ExtractString(body, "token")
		if status != 200 || token == "" {
			return util.NewResult(false, "QR registration should immediately return a session")
		}
		tokens = append(tokens, token)
		status, judge := util.GetRequestWithStatus(context.Logger, "/judge", util.JudgeAuth(token))
		if status != 200 || util.ExtractString(judge, "name") != name {
			return util.NewResult(false, "QR registration did not authenticate the trimmed judge name")
		}
	}
	if tokens[0] == tokens[1] {
		return util.NewResult(false, "QR judges must have separate sessions")
	}
	if countJudgesInList(util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())) != countBefore+2 {
		return util.NewResult(false, "One registration QR should admit multiple judges")
	}

	return util.ResultOk()
}

func countJudgesInList(body string) int {
	var judges []map[string]any
	if err := jsonUnmarshalList(body, &judges); err != nil {
		return 0
	}
	return len(judges)
}
