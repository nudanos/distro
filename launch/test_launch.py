from __future__ import annotations

import unittest

import launch


class DescriptionTest(unittest.TestCase):
    def test_credits_source_and_keeps_original(self):
        d = launch.description({"name": "configd", "description": "YANG config daemon"}, "danos")
        self.assertTrue(d.startswith("Preserved fork of danos/configd (DANOS, dormant since 2021)."))
        self.assertIn("Original: YANG config daemon", d)

    def test_empty_original_and_length_cap(self):
        self.assertNotIn("Original", launch.description({"name": "x", "description": None}, "danos"))
        self.assertLessEqual(len(launch.description({"name": "x", "description": "y" * 999}, "danos")), 350)


class PushPlanTest(unittest.TestCase):
    def test_small_repo_is_one_push_of_heads_and_tags(self):
        plan = launch.push_plan(["refs/heads/master"], ["refs/tags/v1"], False, {})
        self.assertEqual(plan, [["+refs/heads/*:refs/heads/*", "refs/tags/*:refs/tags/*"]])

    def test_small_repo_without_tags(self):
        self.assertEqual(launch.push_plan(["refs/heads/m"], [], False, {}), [["+refs/heads/*:refs/heads/*"]])

    def test_empty_repo_has_nothing_to_push(self):
        self.assertEqual(launch.push_plan([], [], False, {}), [])

    def test_big_repo_pushes_history_in_chunks_then_tags_in_batches(self):
        commits = [f"c{i}" for i in range(12000)]
        tags = [f"refs/tags/t{i}" for i in range(650)]
        plan = launch.push_plan(["refs/heads/main"], tags, True, {"refs/heads/main": commits})
        self.assertEqual(plan[0], ["c4999:refs/heads/main"])
        self.assertEqual(plan[1], ["c9999:refs/heads/main"])
        self.assertEqual(plan[2], ["refs/heads/main:refs/heads/main"])
        self.assertEqual([len(b) for b in plan[3:]], [300, 300, 50])
        self.assertNotIn("refs/pull", repr(plan))


class PendingTest(unittest.TestCase):
    def test_resume_skips_done_and_retries_failed(self):
        state = {"a": "done", "b": "failed"}
        self.assertEqual(launch.pending(["c", "a", "b"], state, None), ["b", "c"])

    def test_only_filter(self):
        self.assertEqual(launch.pending(["a", "b", "c"], {}, ["c", "a"]), ["a", "c"])


if __name__ == "__main__":
    unittest.main()
