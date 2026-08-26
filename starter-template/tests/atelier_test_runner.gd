extends SceneTree

const REPORT_PREFIX := "CODEX_GAME_ATELIER_TEST_REPORT "
const GAME_STATE = preload("res://scripts/game_state.gd")
const LOCALIZED_FIXTURE = preload("res://中文 资源/status_payload.gd")

var _tests: Array[Dictionary] = []


func _initialize() -> void:
	_test_scene_and_ui()
	_test_localized_resource()
	_test_signal_flow()
	_test_input_action()
	_test_gameplay_state()

	var failed := 0
	for test: Dictionary in _tests:
		if test["outcome"] == "FAIL":
			failed += 1
	var outcome := "PASS" if failed == 0 else "FAIL"
	print(REPORT_PREFIX + JSON.stringify({
		"schema_version": "1.0.0",
		"outcome": outcome,
		"tests": _tests,
	}))
	quit(0 if failed == 0 else 1)


func _test_scene_and_ui() -> void:
	var scene := load("res://main.tscn") as PackedScene
	var root := scene.instantiate() if scene != null else null
	var status := root.get_node_or_null("Interface/Panel/Margin/Content/Status") if root != null else null
	var button := root.get_node_or_null("Interface/Panel/Margin/Content/PlayButton") if root != null else null
	_record("starter-scene-ui", root != null and status is Label and button is Button, "The starter scene and basic UI controls instantiate.")
	if root != null:
		root.free()


func _test_localized_resource() -> void:
	_record("localized-resource", LOCALIZED_FIXTURE.value() == "中文与空格路径已加载", "A localized resource path loads correctly.")


func _test_signal_flow() -> void:
	var emitter := Node.new()
	emitter.add_user_signal("activated")
	var observed := [false]
	emitter.connect("activated", func() -> void: observed[0] = true)
	emitter.emit_signal("activated")
	_record("signal-flow", observed[0], "A signal connects and emits synchronously.")
	emitter.free()


func _test_input_action() -> void:
	const ACTION := "atelier_starter_test_action"
	if InputMap.has_action(ACTION):
		InputMap.erase_action(ACTION)
	InputMap.add_action(ACTION)
	var present := InputMap.has_action(ACTION)
	InputMap.erase_action(ACTION)
	_record("input-action", present and not InputMap.has_action(ACTION), "An input action can be added, observed, and removed in memory.")


func _test_gameplay_state() -> void:
	var state := GAME_STATE.new()
	state.add_points(7)
	state.add_points(5)
	_record("gameplay-state", state.score == 12, "The starter gameplay state updates deterministically.")


func _record(id: String, passed: bool, summary: String) -> void:
	_tests.append({
		"id": id,
		"outcome": "PASS" if passed else "FAIL",
		"summary": summary,
	})
