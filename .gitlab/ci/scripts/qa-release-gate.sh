# Sourced from the SaaS QA include (see .gitlab/ci/.gitlab-saas-qa.yml), by both the build
# and the sync-branch job, at `custom_script_position: script_before`.
#
# QA_GATE_ACTION selects what happens when the gate decides to skip:
#   * cancel (default) - call /pipelines/:id/cancel and exit 0. The script step
#     (buildx push) starts but SIGTERM arrives almost immediately from the
#     cancellation, so the job ends as "canceled" (gray) instead of "failed"
#     (red). The pipeline keeps deploy from running two ways: when this gate
#     cancels it, deploy is never created; when build fails for non-gate reasons
#     (no allow_failure), deploy's `when: on_success` + `needs` skips it. So no
#     failure email unless build actually fails.
#   * skip - exit 0 without touching the pipeline. The job stops before its own
#     work and the pipeline keeps running. The sync job uses this: it must not
#     merge main into qa, but it must not take the main pipeline down either.
#     In skip mode the cutoff also freezes the job: from the cutoff on, no source
#     (push, schedule or web) may trigger the sync any more.
#
# QA cadence follows the open milestone's release week. The cutoff is 18:00 CST on the
# most recent Wednesday on or before the milestone due date:
#   * before the cutoff: a scheduled pipeline deploys, pushes do not (nightly cadence);
#   * from the cutoff on: pushes deploy, the schedule stops (real-time cadence).
# `rules` cannot express this because they are evaluated when the pipeline is created,
# so both cadences create the jobs and this gate decides at runtime. The schedule can
# stay enabled permanently: the cutoff moves forward on its own when the next milestone
# opens.
#
# A manual run (web) is not gated at all, so it deploys whatever it is pointed at
# without depending on the milestones API.
#
# Reads group 110 milestones and cancels the pipeline with
# STARHUB_GROUP_CI_TOKEN, which needs `api` scope on that group (cancel is a
# write). $CI_JOB_TOKEN cannot be used: job tokens are not accepted by
# group-level endpoints.
#
# Three exit paths: the skip action + exit 0 for the time gate (gray for cancel,
# green for skip); exit 1 for infra errors (token missing, API down, parse
# failure, or cancel API failure — red, with email); return for a cancel-mode
# caller outside qa, web source, or no open milestone (the job runs). A cancel
# failure must surface: if it returns 1 and we kept going, build would push and
# deploy would run anyway, silently losing the gate's skip decision.
#
# Every path prints a `qa gate: decision=ALLOW|CANCEL|ERROR reason=...` line,
# preceded by the inputs that produced it (source, branch, now, active
# milestones, chosen due date and cutoff), so the job log alone explains why a
# pipeline was gated or not.

cancel_pipeline() {
  echo "qa gate: canceling pipeline $CI_PIPELINE_ID"
  if ! curl --silent --fail --max-time 10 --request POST \
    --header "PRIVATE-TOKEN: $STARHUB_GROUP_CI_TOKEN" \
    "$CI_API_V4_URL/projects/$CI_PROJECT_ID/pipelines/$CI_PIPELINE_ID/cancel"; then
    echo "qa gate: cancel pipeline request failed"
    return 1
  fi
  echo "qa gate: pipeline $CI_PIPELINE_ID canceled"
}

# Runs the action the gate decided on. In skip mode nothing is canceled: the caller
# exits instead, so the job stops before its own work and the pipeline keeps running.
skip_action() {
  if [ "$QA_GATE_ACTION" = "skip" ]; then
    echo "qa gate: action=skip reason=the job stops before its work, the pipeline keeps running"
    return 0
  fi
  echo "qa gate: action=cancel"
  cancel_pipeline
}

qa_release_gate() {
  echo "qa gate: source=$CI_PIPELINE_SOURCE branch=$CI_COMMIT_BRANCH pipeline=$CI_PIPELINE_ID"
  echo "qa gate: now=$(TZ=CST-8 date +'%F %T') CST"

  # cancel targets $CI_PIPELINE_ID, so a cancel-mode caller on main would take down the
  # whole main pipeline, including build-stage-saas and the staging deploy chain. Only
  # qa pipelines may be canceled; the sync job runs on main and uses skip mode, which
  # never touches the pipeline.
  if [ "$QA_GATE_ACTION" != "skip" ] && [ "$CI_COMMIT_BRANCH" != "qa" ]; then
    echo "qa gate: decision=ALLOW reason=cancel is restricted to the qa branch, branch is '$CI_COMMIT_BRANCH'"
    return
  fi

  # A manual run is never gated in cancel mode. In skip mode it still has to respect the
  # cutoff: once the release candidate is frozen, nothing may trigger the sync any more.
  if [ "$CI_PIPELINE_SOURCE" = "web" ] && [ "$QA_GATE_ACTION" != "skip" ]; then
    echo "qa gate: decision=ALLOW reason=manual run (web) is never gated"
    return
  fi

  if [ -z "$STARHUB_GROUP_CI_TOKEN" ]; then
    echo "qa gate: decision=ERROR reason=STARHUB_GROUP_CI_TOKEN is not set"
    exit 1
  fi

  NOW=$(date +%s)   # absolute, so the comparison does not depend on the runner's timezone
  TODAY=$(TZ=CST-8 date +%F)   # CST-8 is UTC+8 and needs no tzdata

  MILESTONES=$(curl --silent --fail \
    --header "PRIVATE-TOKEN: $STARHUB_GROUP_CI_TOKEN" \
    "$CI_API_V4_URL/groups/110/milestones?state=active") || {
    echo "qa gate: decision=ERROR reason=cannot list group 110 milestones"
    exit 1
  }

  # List every active milestone so the selected due date is explainable.
  printf '%s' "$MILESTONES" \
    | jq -r '.[] | "qa gate: active milestone \(.title): \(.start_date) .. \(.due_date)"' 2>/dev/null || true

  # Overlapping milestones resolve to the nearest due date.
  DUE=$(printf '%s' "$MILESTONES" | jq -r --arg today "$TODAY" '
    [ .[] | select(.start_date <= $today and .due_date >= $today) | .due_date ]
    | sort | first // empty
  ') || {
    echo "qa gate: decision=ERROR reason=cannot parse the milestones response"
    exit 1
  }

  if [ -z "$DUE" ]; then
    echo "qa gate: decision=ALLOW reason=no active milestone covers $TODAY"
    return
  fi

  # jq's weekday is 0=Sunday, so (wday + 4) % 7 days before the due date is the most
  # recent Wednesday on or before it. jq's mktime reads the date as UTC and 18:00 CST
  # is 10:00 UTC, so 36000 seconds brings the midnight to the cutoff.
  CUTOFF=$(jq -rn --arg due "$DUE" '
    ($due | strptime("%Y-%m-%d") | mktime) as $w
    | ($w - ((((($w | gmtime)[6]) + 4) % 7) * 86400) + 36000) as $c
    | "\($c) \(($c + 28800) | strftime("%Y-%m-%d %H:%M"))"
  ') || {
    echo "qa gate: decision=ERROR reason=cannot parse due date '$DUE'"
    exit 1
  }
  CUTOFF_EPOCH=${CUTOFF%% *}
  CUTOFF_LOCAL=${CUTOFF#* }

  echo "qa gate: milestone due=$DUE cutoff=$CUTOFF_LOCAL CST"

  if [ "$QA_GATE_ACTION" = "skip" ]; then
    if [ "$NOW" -ge "$CUTOFF_EPOCH" ]; then
      echo "qa gate: decision=CANCEL reason=past the cutoff, the sync is frozen"
      skip_action || exit 1
      exit 0
    fi
    echo "qa gate: decision=ALLOW reason=before the cutoff (qa keeps tracking main)"
    return
  fi

  case "$CI_PIPELINE_SOURCE" in
    schedule)
      if [ "$NOW" -ge "$CUTOFF_EPOCH" ]; then
        echo "qa gate: decision=CANCEL reason=schedule past the cutoff"
        skip_action || exit 1
        exit 0
      fi
      echo "qa gate: decision=ALLOW reason=schedule before the cutoff (nightly cadence)"
      ;;
    push)
      if [ "$NOW" -lt "$CUTOFF_EPOCH" ]; then
        echo "qa gate: decision=CANCEL reason=push before the cutoff"
        skip_action || exit 1
        exit 0
      fi
      echo "qa gate: decision=ALLOW reason=push past the cutoff (real-time cadence)"
      ;;
  esac
}

qa_release_gate
