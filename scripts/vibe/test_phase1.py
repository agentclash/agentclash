import copy
import json
from pathlib import Path
import tempfile
import unittest

import capture_baseline as capture
import quality
import run_baseline
from db_env import connection_env


class ReferenceTests(unittest.TestCase):
    def setUp(self):
        self.corpus=quality.load_corpus()
        self.policy=json.loads((quality.DATA/'quality-policy.json').read_text())

    def predictions(self,corpus=None):
        return [{"id":c["id"],"status":"ok","label":c["expected"]["label"],"source_ids":c["expected"].get("source_ids",[]),
                 "turn_ms":100,"cost_nano_usd":1000,"asked_question":c["expected"].get("question_needed",False),
                 "suite_completed":c["expected"].get("suite_expected",False)}
                for c in (corpus or self.corpus)["cases"] if c["split"]=="holdout"]

    def reviewed_fixture(self):
        # Test-only attestation to exercise the gate, never written to the corpus.
        corpus=copy.deepcopy(self.corpus)
        for c in corpus['cases']:
            c['review']={'status':'human_reviewed','reviewer':'unit-test-only','reviewed_at':'2026-09-22T00:00:00Z','case_sha256':quality.review_digest(c)}
        return corpus

    def test_real_references_are_explicitly_unreviewed(self):
        self.assertEqual(len(self.corpus['cases']),70)
        self.assertFalse(any(quality.reviewed(c) for c in self.corpus['cases']))
        self.assertEqual(set(c['track'] for c in self.corpus['cases']),set(quality.TRACKS))

    def test_gold_is_never_exported_to_model(self):
        before=quality.model_inputs(self.corpus,'holdout')
        changed=copy.deepcopy(self.corpus)
        for c in changed['cases']:
            c['expected']={'label':'GOLD_SENTINEL'};c['review']={'status':'GOLD_SENTINEL'}
        self.assertEqual(before,quality.model_inputs(changed,'holdout'))
        self.assertNotIn('GOLD_SENTINEL',json.dumps(before))
        for row in before:
            self.assertRegex(row['id'],r'^[a-f0-9]{24}$')

    def test_opaque_exported_ids_score_and_alias_duplicates_fail(self):
        rows=self.predictions()
        cases={c['id']:c for c in self.corpus['cases']}
        opaque=[{**r,'id':quality.request_id(cases[r['id']])} for r in rows]
        report=quality.score(self.corpus,opaque,self.policy)
        self.assertEqual(sum(t['correct'] for t in report['tracks'].values()),36)
        with self.assertRaises(ValueError):quality.score(self.corpus,[*opaque,rows[0]],self.policy)

    def test_perfect_machine_labels_do_not_qualify(self):
        report=quality.score(self.corpus,self.predictions(),self.policy)
        self.assertFalse(report['release_ready'])
        self.assertFalse(report['quality_gate_passed'])
        self.assertTrue(any('human review pending' in b for b in report['blockers']))

    def test_review_is_invalidated_by_expected_or_input_edit(self):
        c=self.reviewed_fixture()['cases'][0];self.assertTrue(quality.reviewed(c))
        c['input']['new_statement']='changed';self.assertFalse(quality.reviewed(c))

    def test_loader_rejects_cross_split_groups_and_stale_reviews(self):
        for change in ('group','review'):
            c=self.reviewed_fixture()
            if change=='group':
                a=next(x for x in c['cases'] if x['split']=='development')
                b=next(x for x in c['cases'] if x['split']=='holdout');b['group']=a['group']
            else:c['cases'][0]['review']['case_sha256']='wrong'
            with tempfile.TemporaryDirectory() as tmp:
                p=Path(tmp)/'cases.json';p.write_text(json.dumps(c))
                with self.assertRaises(ValueError):quality.load_corpus(p)

    def test_failure_and_missing_prediction_stay_in_denominator(self):
        rows=self.predictions();rows[0]['status']='error';rows.pop()
        report=quality.score(self.corpus,rows,self.policy)
        self.assertEqual(sum(v['cases'] for v in report['tracks'].values()),36)
        self.assertEqual(sum(v['correct'] for v in report['tracks'].values()),34)
        self.assertFalse(report['release_ready'])

    def test_duplicate_unknown_predictions_and_invalid_values_rejected(self):
        rows=self.predictions()
        for changed in ([*rows,rows[0]],[{**rows[0],'id':'unknown'}], [{**rows[0],'turn_ms':float('nan')}],
                        [{**rows[0],'cost_nano_usd':-1}], [{**rows[0],'turn_ms':True}], [{**rows[0],'label':[]}],
                        [{**rows[0],'prompt':'private'}]):
            with self.assertRaises(ValueError):quality.score(self.corpus,changed,self.policy)

    def test_source_selection_must_match_exactly(self):
        rows=self.predictions();rows[0]['source_ids']=['old-joke']
        report=quality.score(self.corpus,rows,self.policy)
        self.assertEqual(report['tracks']['interpretation']['source_errors'],1)
        self.assertEqual(report['tracks']['interpretation']['correct'],11)

    def test_false_acceptance_and_missing_telemetry_are_visible(self):
        rows=self.predictions();r=next(r for r in rows if r['label']=='contradicted');r['label']='supported';r['cost_nano_usd']=None
        report=quality.score(self.corpus,rows,self.policy)
        self.assertEqual(report['tracks']['test_validity']['false_accepts'],1)
        self.assertEqual(report['tracks']['test_validity']['unknown_cost_cases'],1)

    def test_baseline_required_and_latency_cannot_regress_silently(self):
        c=self.reviewed_fixture();rows=self.predictions(c);first=quality.score(c,rows,self.policy)
        self.assertTrue(first['quality_gate_passed']);self.assertFalse(first['release_ready'])
        self.assertTrue(quality.score(c,rows,self.policy,baseline=first)['release_ready'])
        for r in rows:r['turn_ms']=300
        report=quality.score(c,rows,self.policy,baseline=first)
        self.assertFalse(report['release_ready'])
        self.assertTrue(any('latency regression' in b for b in report['blockers']))

    def test_mismatched_baseline_and_development_cannot_release(self):
        c=self.reviewed_fixture();rows=self.predictions(c);baseline=quality.score(c,rows,self.policy)
        baseline['corpus_sha256']='other'
        self.assertFalse(quality.score(c,rows,self.policy,baseline=baseline)['release_ready'])
        self.assertFalse(quality.score(c,[],self.policy,split='development')['release_ready'])

    def test_duplicate_json_fields_rejected(self):
        with self.assertRaises(ValueError):quality.strict_load('{"version":1,"version":2}')


class DatabaseGateTests(unittest.TestCase):
    def events(self):
        return [{'Package':p,'Test':t,'Action':'pass'} for p,names in run_baseline.REQUIRED.items() for t in names]
    def test_required_tests_must_actually_pass(self):
        self.assertEqual(run_baseline.check_events(self.events(),0),[])
        for state in ('skip','fail'):
            events=self.events();events[0]['Action']=state
            self.assertTrue(run_baseline.check_events(events,0))
        self.assertTrue(run_baseline.check_events(self.events()[:-1],0))
        self.assertTrue(run_baseline.check_events(self.events(),1))
    def test_parent_pass_does_not_hide_skipped_child(self):
        events=self.events();events.append({**events[0],'Test':events[0]['Test']+'/database','Action':'skip'})
        self.assertTrue(run_baseline.check_events(events,0))
    def test_connection_credentials_are_environment_only(self):
        env=connection_env('postgres://test:p%40ss@127.0.0.1:5432/vibe_test?sslmode=disable')
        self.assertEqual(env['PGPASSWORD'],'p@ss')
        self.assertEqual(env['PGDATABASE'],'vibe_test')
        self.assertEqual(env['PGHOST'],'127.0.0.1')
    def test_nonlocal_connection_rejected(self):
        with self.assertRaises(ValueError):connection_env('postgres://test:pass@production.example/db')


class CaptureTests(unittest.TestCase):
    def row(self):return {'operation_id':'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','authoring_version':11,'kind':'message','state':'FAILED','turn_ms':100,'cost_nano_usd':None,'model_calls':1}
    def test_error_and_unknown_cost_are_not_hidden(self):
        result=capture.summarize([self.row()])['11/message']
        self.assertEqual(result['turns'],1);self.assertEqual(result['unknown_cost_turns'],1)
        self.assertEqual(result['states']['FAILED'],1)
        self.assertIsNone(result['unnecessary_questions']);self.assertIsNone(result['useful_suite_completion'])
    def test_no_raw_text_fields_or_duplicate_events(self):
        with self.assertRaises(ValueError):capture.summarize([{**self.row(),'prompt':'private'}])
        with self.assertRaises(ValueError):capture.summarize([self.row(),self.row()])
    def test_summary_has_no_operation_identifier(self):
        self.assertNotIn(self.row()['operation_id'],json.dumps(capture.summarize([self.row()])))

if __name__=='__main__':unittest.main()
