/** Which panel is currently displayed — pure local UI navigation, decoupled
 * from what's actually running (see statemachine.Status.subMode for that). */
export type PanelKey = 'off' | 'fast' | 'private'
