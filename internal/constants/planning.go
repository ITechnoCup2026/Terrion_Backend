package constants

type PlanningObjective string

const (
	ObjectiveSafe   PlanningObjective = "aman"
	ObjectiveIncome PlanningObjective = "pendapatan"
	ObjectiveMarket PlanningObjective = "pasar"
)

type PlanStatus string

const (
	PlanApplied   PlanStatus = "applied"
	PlanCancelled PlanStatus = "cancelled"
)

const (
	PlanNoPlots              = "plan_no_plots"
	PlanNoClimateNormals     = "plan_no_climate_normals"
	PlanSeasonClosed         = "plan_season_closed"
	PlanNoEligiblePlots      = "plan_no_eligible_plots"
	PlanAlreadyApplied       = "plan_already_applied"
	PlanAlreadyCancelled     = "plan_already_cancelled"
	PlanNotFound             = "plan_not_found"
	PlanAssignmentRejected   = "plan_assignment_rejected"
	PlanPartiallyCancellable = "plan_partially_cancellable"
	PlanGoalTooLong          = "plan_goal_too_long"
)

const (
	PlanningLocalSearchPasses = 3
	PlanningEvaluationBudget  = 200000
)

// PlanGoalMaxChars adalah batas panjang tujuan bahasa bebas dari pengurus,
// disamakan dengan MAX_GOAL_CHARS pada kontrak layanan AI. Batas itu
// ditegakkan di sini supaya kalimat kepanjangan ditolak sebagai tujuan yang
// terlalu panjang, bukan sebagai permintaan rencana yang gagal.
const PlanGoalMaxChars = 500

// PlanClimateDisclaimer adalah satu baris yang wajib ikut setiap proposal.
// Jendela panen rencana dihitung dari normal iklim, bukan dari ramalan cuaca —
// karena cuaca musim depan memang belum terjadi. Kalimatnya tetap dan tidak
// bergantung data, jadi ia tinggal di sini dan bukan dirangkai di setiap tempat.
const PlanClimateDisclaimer = "Rencana ini dihitung dari iklim rata-rata " +
	"sepuluh tahun. Cuaca musim depan belum terjadi."
