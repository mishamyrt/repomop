use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};
use ratatui::layout::Rect;

use crate::app::{App, ViewState};
use crate::widgets::max_confirm_offset;

impl App {
    pub(crate) fn handle_key(&mut self, key: KeyEvent, area: Rect) {
        match self.state {
            ViewState::Loading => {
                if is_quit_key(key) {
                    self.should_quit = true;
                }
            }
            ViewState::List => self.handle_list_key(key),
            ViewState::Confirm => self.handle_confirm_key(key, area),
            ViewState::Deleting => {}
            ViewState::Done | ViewState::Error => {
                if is_quit_key(key) || key.code == KeyCode::Enter {
                    self.should_quit = true;
                }
            }
        }
    }

    fn handle_list_key(&mut self, key: KeyEvent) {
        match key.code {
            KeyCode::Up | KeyCode::Char('k') => {
                self.cursor = self.cursor.saturating_sub(1);
            }
            KeyCode::Down | KeyCode::Char('j') => {
                if self.cursor + 1 < self.artifacts.len() {
                    self.cursor += 1;
                }
            }
            KeyCode::Char(' ') => {
                if self.artifacts.is_empty() {
                    return;
                }
                if !self.selected.insert(self.cursor) {
                    self.selected.remove(&self.cursor);
                }
            }
            KeyCode::Enter => {
                if self.selected.is_empty() {
                    return;
                }
                self.cached_selection = Some(self.selected_artifacts());
                self.confirm_offset = 0;
                self.message.clear();
                self.state = ViewState::Confirm;
            }
            _ if is_quit_key(key) => self.should_quit = true,
            _ => {}
        }
    }

    fn handle_confirm_key(&mut self, key: KeyEvent, area: Rect) {
        let selected_len = self.cached_selection.as_ref().map(Vec::len).unwrap_or(0);
        match key.code {
            KeyCode::Esc | KeyCode::Char('n') => {
                self.cached_selection = None;
                self.confirm_offset = 0;
                self.state = ViewState::List;
            }
            KeyCode::Up | KeyCode::Char('k') => {
                self.confirm_offset = self.confirm_offset.saturating_sub(1);
            }
            KeyCode::Down | KeyCode::Char('j') => {
                let max_offset =
                    max_confirm_offset(selected_len, area.height as usize);
                if self.confirm_offset < max_offset {
                    self.confirm_offset += 1;
                }
            }
            KeyCode::Char('y') => {
                if let Some(selected) = self.cached_selection.take() {
                    self.confirm_offset = 0;
                    if selected.is_empty() {
                        self.state = ViewState::List;
                    } else {
                        self.message.clear();
                        self.state = ViewState::Deleting;
                        self.spawn_delete(selected);
                    }
                }
            }
            KeyCode::Enter => {}
            _ if is_quit_key(key) => self.should_quit = true,
            _ => {}
        }
    }
}

pub(crate) fn is_quit_key(key: KeyEvent) -> bool {
    matches!(key.code, KeyCode::Esc | KeyCode::Char('q'))
        || (key.code == KeyCode::Char('c')
            && key.modifiers.contains(KeyModifiers::CONTROL))
}
