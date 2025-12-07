package tui

func (m Model) handleNavigationInput(key string) Model {
	switch key {
	case KeyUp, KeyK:
		m = m.moveUp()
	case KeyDown, KeyJ:
		m = m.moveDown()
	case KeyLeft, KeyH:
		m = m.moveToPreviousColumn()
	case KeyRight, KeyL:
		m = m.moveToNextColumn()
	}
	return m
}

func (m Model) moveUp() Model {
	switch m.focused {
	case CommandColumn:
		if m.selectedCommand > FirstItemIndex {
			m.selectedCommand--
		}
	case StackColumn:
		if m.selectedStack > FirstItemIndex {
			m.selectedStack--
		}
	}
	return m
}

func (m Model) moveDown() Model {
	switch m.focused {
	case CommandColumn:
		if m.selectedCommand < len(m.commands)-1 {
			m.selectedCommand++
		}
	case StackColumn:
		if m.selectedStack < len(m.stacks)-1 {
			m.selectedStack++
		}
	}
	return m
}

func (m Model) moveToPreviousColumn() Model {
	if m.focused == CommandColumn {
		m.focused = StackColumn
	} else {
		m.focused = CommandColumn
	}
	return m
}

func (m Model) moveToNextColumn() Model {
	if m.focused == StackColumn {
		m.focused = CommandColumn
	} else {
		m.focused = StackColumn
	}
	return m
}
