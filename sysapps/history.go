package sysapps

const MaxHistory = 50

type History interface {
	AddCard()
	RemoveCard()
}

type DefaultHistory struct{}
