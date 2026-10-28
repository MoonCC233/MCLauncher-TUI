package i18n

// catalogs holds the translation tables. English is implicit: the English
// key itself is the default text. Each catalog is defined in its own file so
// translators can work on one language without touching the others.
var catalogs = map[Lang]map[string]string{
	ES:     esCatalog,
	FR:     frCatalog,
	DE:     deCatalog,
	ZHHans: zhHansCatalog,
	ZHHant: zhHantCatalog,
	JA:     jaCatalog,
	AR:     arCatalog,
	PTBR:   ptBRCatalog,
	PTPT:   ptPTCatalog,
	RU:     ruCatalog,
	KO:     koCatalog,
	TR:     trCatalog,
	VI:     viCatalog,
}
