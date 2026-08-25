extends SceneTree

const REPORT_PREFIX := "CODEX_GAME_ATELIER_TEST_REPORT "
const STATUS_PAYLOAD = preload("res://中文 资源/status_payload.gd")
const GAMEPLAY_STATE = preload("res://gameplay_state.gd")

var _tests: Array[Dictionary] = []


func _initialize() -> void:
	_test_localized_resource()
	_test_signal_flow()
	_test_input_action()
	_test_ui_state()
	_test_gameplay_state()

	var failed := 0
	for test: Dictionary in _tests:
		if test["outcome"] == "FAIL":
			failed += 1
	var outcome := "PASS" if failed == 0 else "FAIL"
	var report := {
		"schema_version": "1.0.0",
		"outcome": outcome,
		"tests": _tests,
	}
	print(REPORT_PREFIX + JSON.stringify(report))
	quit(0 if failed == 0 else 1)


func _test_localized_resource() -> void:
	_record("localized-resource", STATUS_PAYLOAD.value() == "中文与空格路径已加载", "A GDScript resource with Chinese and space path segments loads correctly.")


func _test_signal_flow() -> void:
	var emitter := Node.new()
	emitter.add_user_signal("activated")
	var observed := [false]
	emitter.connect("activated", func() -> void: observed[0] = true)
	emitter.emit_signal("activated")
	_record("signal-flow", observed[0], "A runtime signal connects and emits synchronously.")
	emitter.free()


func _test_input_action() -> void:
	const ACTION := "atelier_test_action"
	if InputMap.has_action(ACTION):
		InputMap.erase_action(ACTION)
	InputMap.add_action(ACTION)
	var present := InputMap.has_action(ACTION)
	InputMap.erase_action(ACTION)
	_record("input-action", present and not InputMap.has_action(ACTION), "An input action can be added, observed, and removed in memory.")


func _test_ui_state() -> void:
	var label := Label.new()
	label.text = "中文 UI"
	_record("ui-state", label.text == "中文 UI" and label is Control, "A basic UI control preserves localized text and Control identity.")
	label.free()


func _test_gameplay_state() -> void:
	var state := GAMEPLAY_STATE.new()
	state.add_points(7)
	state.add_points(5)
	_record("gameplay-state", state.score == 12, "The reference gameplay state applies deterministic score updates.")


func _record(id: String, passed: bool, summary: String) -> void:
	_tests.append({
		"id": id,
		"outcome": "PASS" if passed else "FAIL",
		"summary": summary,
	})
