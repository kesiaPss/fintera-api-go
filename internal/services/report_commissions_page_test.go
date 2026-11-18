package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func commissionPageFixture() []CommissionReportItem {
	paidAt := time.Now()
	ana := uint(7)
	carlos := uint(8)
	return []CommissionReportItem{
		{SellerID: &ana, Seller: "Ana", Amount: 100, Commission: 10, CommissionPaidAt: &paidAt},
		{SellerID: &ana, Seller: "Ana", Amount: 300, Commission: 30},
		{SellerID: &carlos, Seller: "Carlos", Amount: 0, Commission: 5},
		{Seller: "Unknown", Amount: 50, Commission: 2},
	}
}

func TestCommissionTotalsForFilteredRows(t *testing.T) {
	totals := CommissionTotalsFor(commissionPageFixture(), 4)

	assert.Equal(t, 47.0, totals.Commission)
	assert.Equal(t, 450.0, totals.Amount)
	assert.Equal(t, 10.0, totals.Paid)
	assert.Equal(t, 37.0, totals.Pending)
	assert.Equal(t, 1, totals.PaidCount)
	assert.InDelta(t, 47.0/450.0*100, totals.AverageRate, 1e-9)
	assert.Equal(t, 33.0, totals.NetPending)
}

func TestCommissionTotalsAverageRateZeroWithoutAmount(t *testing.T) {
	items := []CommissionReportItem{{Commission: 5}}

	totals := CommissionTotalsFor(items, 0)

	assert.Equal(t, 0.0, totals.AverageRate)
	assert.Equal(t, 5.0, totals.Pending)
}

func TestCommissionSellersForSortsAndSkipsRowsWithoutSeller(t *testing.T) {
	sellers := CommissionSellersFor(commissionPageFixture())

	assert.Equal(t, []CommissionSeller{{ID: 7, Name: "Ana"}, {ID: 8, Name: "Carlos"}}, sellers)
}

func TestFilterCommissionsBySellerExcludesRowsWithoutSellerID(t *testing.T) {
	items := commissionPageFixture()

	matched := FilterCommissionsBySeller(items, 7)
	assert.Len(t, matched, 2)
	for _, item := range matched {
		assert.Equal(t, uint(7), *item.SellerID)
	}

	none := FilterCommissionsBySeller(items, 99)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

func commissionSearchFixture() []CommissionReportItem {
	historicalID := uint(99)
	return []CommissionReportItem{
		{ContractID: 43, ClientName: "María López", Project: "Proyecto Norte", Lot: "Lote 7", Seller: "Ana Vendedora"},
		{ContractID: 100, ClientName: "Carlos Ruiz", Project: "Proyecto Sur", Lot: "Lote 12", Seller: "Carlos Vendedor"},
		{HistoricalSaleID: &historicalID, IsHistorical: true, ClientName: "Importado", Project: "N/A", Lot: "N/A", Seller: "Ana Vendedora"},
	}
}

func TestFilterCommissionsBySearchMatchesClientCaseInsensitive(t *testing.T) {
	items := commissionSearchFixture()

	matched := FilterCommissionsBySearch(items, "MARÍA")
	assert.Len(t, matched, 1)
	assert.Equal(t, uint(43), matched[0].ContractID)
}

func TestFilterCommissionsBySearchMatchesSeller(t *testing.T) {
	items := commissionSearchFixture()

	matched := FilterCommissionsBySearch(items, "ana vendedora")
	assert.Len(t, matched, 2)
	assert.Equal(t, uint(43), matched[0].ContractID)
	assert.True(t, matched[1].IsHistorical)
}

func TestFilterCommissionsBySearchMatchesContractWithAndWithoutHash(t *testing.T) {
	items := commissionSearchFixture()

	withHash := FilterCommissionsBySearch(items, "#43")
	require.Len(t, withHash, 1)
	assert.Equal(t, uint(43), withHash[0].ContractID)

	withoutHash := FilterCommissionsBySearch(items, "43")
	require.Len(t, withoutHash, 1)
	assert.Equal(t, uint(43), withoutHash[0].ContractID)
}

func TestFilterCommissionsBySearchMatchesHistoricalSaleID(t *testing.T) {
	items := commissionSearchFixture()

	matched := FilterCommissionsBySearch(items, "99")
	require.Len(t, matched, 1)
	require.NotNil(t, matched[0].HistoricalSaleID)
	assert.Equal(t, uint(99), *matched[0].HistoricalSaleID)
}

func TestFilterCommissionsBySearchBlankTermKeepsEveryRow(t *testing.T) {
	items := commissionSearchFixture()

	for _, term := range []string{"", "   "} {
		assert.Equal(t, items, FilterCommissionsBySearch(items, term))
	}
}

func TestFilterCommissionsBySearchKeepsOriginalOrder(t *testing.T) {
	items := commissionSearchFixture()

	matched := FilterCommissionsBySearch(items, "proyecto")
	require.Len(t, matched, 2)
	assert.Equal(t, uint(43), matched[0].ContractID)
	assert.Equal(t, uint(100), matched[1].ContractID)
}
