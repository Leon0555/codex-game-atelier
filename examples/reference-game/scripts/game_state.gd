extends RefCounted

signal score_changed(score: int, goal: int)
signal game_won(final_score: int)

const GOAL := 5

var score := 0
var won := false


func collect(points: int = 1) -> bool:
	if won or points <= 0:
		return false
	score = mini(score + points, GOAL)
	score_changed.emit(score, GOAL)
	if score == GOAL:
		won = true
		game_won.emit(score)
	return true


func reset() -> void:
	score = 0
	won = false
	score_changed.emit(score, GOAL)
