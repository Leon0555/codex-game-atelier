extends Node

const STATUS_PAYLOAD = preload("res://中文 资源/status_payload.gd")


func _ready() -> void:
	var result := {
		"event": "atelier_reference_ready",
		"path_fixture": STATUS_PAYLOAD.value(),
		"status": "PASS",
	}
	print(JSON.stringify(result))
	get_tree().quit(0)
