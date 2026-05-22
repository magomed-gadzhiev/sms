package network

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/smpp-server/smpp-server/internal/storage"
)

// RouteSignature вычисляет стабильный hash от смысловой подписи маршрута:
// (provider_id, route_type, ordered list of (group_index, logic_op, sorted conditions)).
//
// Внутри каждой группы условия сортируются по (type, value) — порядок ввода в UI
// не влияет на signature (две одинаковые правила в разном порядке = дубликат).
// Порядок групп сохраняется (group_index важен — AND/OR композиция различна).
//
// Используется в handler-pre-check'е для детекции смыслового дубликата
// override-маршрута до INSERT'а. После Plan 3 Task 1 (drop uq_cell_provider)
// это единственная защита от дубликатов.
func RouteSignature(item storage.RouteSetItemFull) string {
	var b strings.Builder
	b.WriteString(item.ProviderID.String())
	b.WriteString("|")
	b.WriteString(item.RouteType)
	b.WriteString("|")
	for idx, g := range item.ConditionGroups {
		b.WriteString("g")
		// Используем idx из позиции массива — array order это единственный канонический
		// источник порядка групп. Каллеры (parseItemIn, materializer-load) могут заполнять
		// g.GroupIndex по-разному (от idx до DB-id), но позиция в массиве всегда совпадает
		// со смысловым порядком — гарантирует одинаковую signature для одинаковых правил
		// независимо от того, откуда RouteSetItemFull собран.
		b.WriteString(strconv.Itoa(idx))
		b.WriteString(":")
		b.WriteString(g.LogicOp)
		b.WriteString("(")
		conds := make([]storage.RouteSetCondition, len(g.Conditions))
		copy(conds, g.Conditions)
		sort.Slice(conds, func(i, j int) bool {
			if conds[i].Type != conds[j].Type {
				return conds[i].Type < conds[j].Type
			}
			return conds[i].Value < conds[j].Value
		})
		for _, c := range conds {
			b.WriteString(c.Type)
			b.WriteString("=")
			b.WriteString(c.Value)
			b.WriteString(",")
		}
		b.WriteString(")")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
