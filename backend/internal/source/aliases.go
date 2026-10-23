package source

// Опечатки текста кейса, которые принимаются наравне с именами схемы организаторов.
// Ключи: pice→price, paymentTem→paymentTerm, autoprolong→autoProlong.
// Значения: quater_year→quarter_year, pefomed→performed, not_pefomed→not_performed.
var valueAliases = map[string]string{
	"quater_year": "quarter_year",
	"pefomed":     "performed",
	"not_pefomed": "not_performed",
}
