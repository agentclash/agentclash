from contextlib import redirect_stdout
import http.client
import io
from pathlib import Path
import sys
import unittest
from unittest.mock import MagicMock, patch
import urllib.error

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
import tools

URL = "https://github.com/example/tool/releases/download/v1/tool.tar.gz"


def response(body):
    stream = MagicMock()
    stream.__enter__.return_value.read.return_value = body
    return stream


def http_error(code):
    return urllib.error.HTTPError(URL, code, "fixture", {}, None)


@patch("tools.time.sleep")
class DownloadTest(unittest.TestCase):
    def test_transient_faults_are_tried_again(self, sleep):
        faults = [http_error(503), http.client.RemoteDisconnected("closed")]
        with patch(
            "tools.urllib.request.urlopen", side_effect=[*faults, response(b"tool")]
        ) as urlopen:
            self.assertEqual(tools.download(URL), b"tool")
        self.assertEqual(urlopen.call_count, 3)
        self.assertEqual(sleep.call_count, 2)

    def test_client_errors_fail_without_another_attempt(self, sleep):
        for code in (403, 404):
            with patch(
                "tools.urllib.request.urlopen", side_effect=http_error(code)
            ) as urlopen:
                with self.assertRaises(urllib.error.HTTPError):
                    tools.download(URL)
            self.assertEqual(urlopen.call_count, 1)
        sleep.assert_not_called()

    def test_rate_limit_is_tried_again(self, sleep):
        with patch(
            "tools.urllib.request.urlopen",
            side_effect=[http_error(429), response(b"tool")],
        ):
            self.assertEqual(tools.download(URL), b"tool")
        sleep.assert_called_once()

    def test_persistent_fault_fails_after_the_last_attempt(self, sleep):
        with patch(
            "tools.urllib.request.urlopen", side_effect=TimeoutError()
        ) as urlopen:
            with self.assertRaises(TimeoutError):
                tools.download(URL)
        self.assertEqual(urlopen.call_count, tools.DOWNLOAD_ATTEMPTS)
        self.assertEqual(sleep.call_count, tools.DOWNLOAD_ATTEMPTS - 1)


class TimedTest(unittest.TestCase):
    def test_phase_time_prints_also_after_a_failure(self):
        import maintained

        output = io.StringIO()
        with (
            redirect_stdout(output),
            patch("maintained.time.monotonic", side_effect=[10.0, 13.4, 20.0, 21.0]),
        ):
            with maintained.timed("platform temporal build"):
                pass
            with self.assertRaises(RuntimeError):
                with maintained.timed("platform temporal scan"):
                    raise RuntimeError("PRIVATE_CANARY")
        self.assertEqual(
            output.getvalue().splitlines(),
            [
                "Image check phase: platform temporal build 3s",
                "Image check phase: platform temporal scan 1s",
            ],
        )


if __name__ == "__main__":
    unittest.main()
