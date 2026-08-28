extends Control

const GAME_STATE = preload("res://scripts/game_state.gd")
const STATUS_PAYLOAD = preload("res://中文 资源/status_payload.gd")

@onready var status: Label = $Interface/Panel/Margin/Content/Status
@onready var score_label: Label = $Interface/Panel/Margin/Content/Score
@onready var progress: ProgressBar = $Interface/Panel/Margin/Content/Progress
@onready var play_button: Button = $Interface/Panel/Margin/Content/Actions/PlayButton
@onready var reset_button: Button = $Interface/Panel/Margin/Content/Actions/ResetButton

var game_state := GAME_STATE.new()


func _ready() -> void:
	game_state.score_changed.connect(_on_score_changed)
	game_state.game_won.connect(_on_game_won)
	play_button.pressed.connect(_collect_spark)
	reset_button.pressed.connect(_reset_game)
	_on_score_changed(game_state.score, game_state.GOAL)
	print(JSON.stringify({
		"event": "atelier_vertical_slice_ready",
		"path_fixture": STATUS_PAYLOAD.value(),
		"status": "PASS",
	}))


func _unhandled_input(event: InputEvent) -> void:
	if event.is_action_pressed("ui_accept"):
		_collect_spark()
		get_viewport().set_input_as_handled()


func _collect_spark() -> void:
	game_state.collect()


func _reset_game() -> void:
	game_state.reset()


func _on_score_changed(score: int, goal: int) -> void:
	score_label.text = "Sparks %d / %d" % [score, goal]
	progress.max_value = goal
	progress.value = score
	play_button.disabled = score >= goal
	reset_button.disabled = score == 0
	if score < goal:
		status.text = STATUS_PAYLOAD.value() if score == 0 else "Keep going · 继续收集"


func _on_game_won(final_score: int) -> void:
	status.text = "Complete! %d sparks · 挑战完成" % final_score
