package main

// GoPaste keeps its own catalogs (i18n.go) instead of a MyGo catalog, so there is
// no `mygo generate` step to write the locale imports for it. MyGo still has
// strings of its own — its controls, dialogs, and the standard items it adds to
// the menu bar — and those are in English unless the app imports the locale
// package for each language it declares with SetLanguages. Importing it here
// keeps the menu bar in one language with the rest of the interface.
import _ "github.com/egoist/mygo/locales/zh-Hans"
