/** Which mode's screen is currently shown — pure local UI navigation,
 * decoupled from what's actually running (see statemachine.Status.subMode and
 * the StatusHub for ground truth). */
export type ModeTab = 'fast' | 'private'
