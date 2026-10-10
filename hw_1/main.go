package main

import "strings"

type Item struct {
	name        string
	place       string
	isContainer bool
}

type Door struct {
	open bool
}

type Exit struct {
	direction string
	room      *Room
	door      *Door
}

type Room struct {
	name    string
	items   []*Item
	exits   []*Exit
	onEnter func(p *Player) string
	onLook  func(p *Player) string
}

type Player struct {
	room      *Room
	inv       map[string]*Item
	equipment map[string]*Item
}

var (
	player  *Player
	rooms   map[string]*Room
	targets map[string]func(p *Player, item *Item) string
)

func initGame() {
	rooms = map[string]*Room{}
	targets = map[string]func(player *Player, item *Item) string{}

	door := &Door{open: false}

	tea := &Item{name: "чай", place: "на столе"}
	keys := &Item{name: "ключи", place: "на столе"}
	notes := &Item{name: "конспекты", place: "на столе"}
	backpack := &Item{name: "рюкзак", place: "на стуле", isContainer: true}

	// Вспомогательные функции для комнат
	newRoom := func(name string, items ...*Item) *Room {
		room := &Room{name: name, items: items}
		rooms[name] = room
		return room
	}

	link := func(a *Room, dirA string, b *Room, dirB string, door *Door) {
		a.exits = append(a.exits, &Exit{direction: dirA, room: b, door: door})
		b.exits = append(b.exits, &Exit{direction: dirB, room: a, door: door})
	}

	describeExits := func(room *Room) string {
		names := make([]string, 0, len(room.exits))
		for _, exit := range room.exits {
			names = append(names, exit.direction)
		}
		return strings.Join(names, ", ")
	}

	describeItems := func(items []*Item) string {
		var order []string
		groups := map[string][]string{}
		for _, item := range items {
			if _, ok := groups[item.place]; !ok {
				order = append(order, item.place)
			}
			groups[item.place] = append(groups[item.place], item.name)
		}
		parts := make([]string, 0, len(order))
		for _, place := range order {
			parts = append(parts, place+": "+strings.Join(groups[place], ", "))
		}
		return strings.Join(parts, ", ")
	}

	kitchen := newRoom("кухня", tea)
	corridor := newRoom("коридор")
	room := newRoom("комната", keys, notes, backpack)
	street := newRoom("улица")

	link(kitchen, "коридор", corridor, "кухня", nil)
	link(corridor, "комната", room, "коридор", nil)
	link(corridor, "улица", street, "домой", door)

	kitchen.onEnter = func(p *Player) string {
		return "кухня, ничего интересного. можно пройти - " + describeExits(kitchen)
	}
	corridor.onEnter = func(p *Player) string {
		return "ничего интересного. можно пройти - " + describeExits(corridor)
	}
	room.onEnter = func(p *Player) string {
		return "ты в своей комнате. можно пройти - " + describeExits(room)
	}
	street.onEnter = func(p *Player) string {
		return "на улице весна. можно пройти - " + describeExits(street)
	}

	kitchen.onLook = func(p *Player) string {
		res := "ты находишься на кухне"
		if items := describeItems(kitchen.items); items != "" {
			res += ", " + items
		}
		if hasContainer(p) {
			res += ", надо идти в универ"
		} else {
			res += ", надо собрать рюкзак и идти в универ"
		}
		return res + ". можно пройти - " + describeExits(kitchen)
	}
	corridor.onLook = func(p *Player) string {
		return "ничего интересного. можно пройти - " + describeExits(corridor)
	}
	room.onLook = func(p *Player) string {
		items := describeItems(room.items)
		if items == "" {
			return "пустая комната. можно пройти - " + describeExits(room)
		}
		return items + ". можно пройти - " + describeExits(room)
	}
	street.onLook = func(p *Player) string {
		return "на улице весна. можно пройти - " + describeExits(street)
	}

	targets["дверь"] = func(player *Player, item *Item) string {
		if item.name == "ключи" {
			door.open = true
			return "дверь открыта"
		}
		return "не к чему применить"
	}

	player = &Player{
		room:      kitchen,
		inv:       map[string]*Item{},
		equipment: map[string]*Item{},
	}
}

func handleCommand(command string) string {
	goTo := func(direction string) string {
		for _, e := range player.room.exits {
			if e.direction != direction {
				continue
			}
			if e.door != nil && !e.door.open {
				return "дверь закрыта"
			}
			player.room = e.room
			return e.room.onEnter(player)
		}
		return "нет пути в " + direction
	}

	wear := func(name string) string {
		if _, ok := player.equipment[name]; ok {
			return "вы надели: " + name
		}
		if x, ok := player.inv[name]; ok {
			delete(player.inv, name)
			player.equipment[name] = x
			return "вы надели: " + name
		}
		for i, x := range player.room.items {
			if x.name == name {
				player.room.items = append(player.room.items[:i], player.room.items[i+1:]...)
				player.equipment[name] = x
				return "вы надели: " + name
			}
		}
		return "нет такого"
	}

	take := func(name string) string {
		if !hasContainer(player) {
			return "некуда класть"
		}
		for i, it := range player.room.items {
			if it.name == name {
				player.room.items = append(player.room.items[:i], player.room.items[i+1:]...)
				player.inv[name] = it
				return "предмет добавлен в инвентарь: " + name
			}
		}
		return "нет такого"
	}

	apply := func(nameOfItem, nameOfTarget string) string {
		it, ok := player.inv[nameOfItem]
		if !ok {
			it, ok = player.equipment[nameOfItem]
		}
		if !ok {
			return "нет предмета в инвентаре - " + nameOfItem
		}
		target, ok := targets[nameOfTarget]
		if !ok {
			return "не к чему применить"
		}
		return target(player, it)
	}

	parts := strings.Split(strings.TrimSpace(command), " ")
	switch parts[0] {
	case "осмотреться":
		return player.room.onLook(player)
	case "идти":
		if len(parts) < 2 {
			return "неизвестная команда"
		}
		return goTo(parts[1])
	case "надеть":
		if len(parts) < 2 {
			return "неизвестная команда"
		}
		return wear(parts[1])
	case "взять":
		if len(parts) < 2 {
			return "неизвестная команда"
		}
		return take(parts[1])
	case "применить":
		if len(parts) < 3 {
			return "неизвестная команда"
		}
		return apply(parts[1], parts[2])
	}
	return "неизвестная команда"
}

func hasContainer(p *Player) bool {
	for _, it := range p.equipment {
		if it.isContainer {
			return true
		}
	}
	return false
}
