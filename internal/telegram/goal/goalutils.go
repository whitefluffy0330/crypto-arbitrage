package goal

import "time"

var lastGoalClosedAt time.Time

func MarkGoalClosed() {
	lastGoalClosedAt = time.Now()
}

func GoalClosedRecently() bool {
	return time.Since(lastGoalClosedAt) < time.Minute
}
