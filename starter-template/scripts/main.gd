extends Node

const GAME_STATE = preload("res://scripts/game_state.gd")

@onready var status: Label = $Interface/Panel/Margin/Content/Status
@onready var play_button: Button = $Interface/Panel/Margin/Content/PlayButton

var game_state := GAME_STATE.new()


func _ready() -> void:
	play_button.pressed.connect(_add_point)


func _add_point() -> void:
	game_state.add_points(1)
	status.text = "Score: %d" % game_state.score
