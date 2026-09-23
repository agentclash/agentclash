import unittest

from understanding_report import summarize


class UnderstandingReportTests(unittest.TestCase):
    def test_full_turn_cost_cache_and_failed_denominators(self):
        result = summarize([
            {"mode": "advisory", "assistant_model": "fixture", "completed": True, "turn_ms": 2000,
             "attempts": [{"step": "advisory:signals", "cost": 20, "reserved": 30},
                          {"step": "route", "cost": 50, "reserved": 100, "cached_input_tokens": 12},
                          {"step": "repair", "cost": 50, "reserved": 100, "cached_input_tokens": 0}]},
            {"mode": "advisory", "assistant_model": "fixture", "completed": False, "turn_ms": None, "outcome": "fallback",
             "attempts": [{"step": "advisory:signals", "cost": None, "reserved": 30},
                          {"step": "route", "cost": None, "reserved": 100}]},
        ])["groups"][0]
        self.assertEqual((result["turns"], result["completed"], result["unfinished_turns"]), (2, 1, 1))
        self.assertEqual(result["known_cost_nano_usd"], 120)
        self.assertEqual(result["unresolved_reserved_nano_usd"], 130)
        self.assertEqual(result["conversation_cache_hits"], 1)
        self.assertEqual(result["conversation_cache_known_misses"], 1)
        self.assertEqual(result["conversation_cache_unknown"], 1)
        self.assertEqual(result["turn_p50_ms"], 2000)
        self.assertEqual(result["conversation_attempts"], 3)

    def test_no_data_does_not_claim_success(self):
        self.assertEqual(summarize([])["groups"], [])
