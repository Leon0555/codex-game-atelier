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
	_test_gameplay_loop()
	_test_reset_flow()

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
	var progress := root.get_node_or_null("Interface/Panel/Margin/Content/Progress") if root != null else null
	var play_button := root.get_node_or_null("Interface/Panel/Margin/Content/Actions/PlayButton") if root != null else null
	var reset_button := root.get_node_or_null("Interface/Panel/Margin/Content/Actions/ResetButton") if root != null else null
	_record("starter-scene-ui", root is Control and status is Label and progress is ProgressBar and play_button is Button and reset_button is Button, "The playable scene and its complete UI instantiate.")
	if root != null:
		root.free()


func _test_localized_resource() -> void:
	_record("localized-resource", LOCALIZED_FIXTURE.value() == "中文与空格路径已加载", "A localized resource path loads correctly.")


func _test_signal_flow() -> void:
	var state := GAME_STATE.new()
	var observed := []
	state.score_changed.connect(func(score: int, goal: int) -> void: observed.append([score, goal]))
	state.collect(2)
	_record("signal-flow", observed == [[2, 5]], "Gameplay emits the score and goal through its public signal.")


func _test_input_action() -> void:
	_record("input-action", InputMap.has_action("ui_accept"), "The playable keyboard action is available through Godot's standard input map.")


func _test_gameplay_loop() -> void:
	var state := GAME_STATE.new()
	var victories := []
	state.game_won.connect(func(final_score: int) -> void: victories.append(final_score))
	var first := state.collect()
	var finish := state.collect(4)
	var after_win := state.collect()
	_record("gameplay-loop", first and finish and not after_win and state.score == 5 and state.won and victories == [5], "The collect-to-goal loop reaches one terminal win and rejects extra scoring.")


func _test_reset_flow() -> void:
	var state := GAME_STATE.new()
	state.collect(5)
	state.reset()
	var replayed := state.collect()
	_record("reset-flow", replayed and state.score == 1 and not state.won, "A completed game resets into a playable new round.")


func _record(id: String, passed: bool, summary: String) -> void:
	_tests.append({
		"id": id,
		"outcome": "PASS" if passed else "FAIL",
		"summary": summary,
	})
