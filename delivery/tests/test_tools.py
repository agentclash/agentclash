from contextlib import redirect_stdout
import http.client
import io
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import MagicMock, patch
import urllib.error

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(ROOT / "delivery"), str(ROOT / "deploy/aws/scripts")]
from common import Refused
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
        other = "https://github.com/example/tool/archive/v1.tar.gz"
        with patch(
            "tools.urllib.request.urlopen", side_effect=TimeoutError()
        ) as urlopen:
            with self.assertRaises(TimeoutError):
                tools.download(other)
        self.assertEqual(urlopen.call_count, tools.DOWNLOAD_ATTEMPTS)
        self.assertEqual(sleep.call_count, tools.DOWNLOAD_ATTEMPTS - 1)

    def test_release_asset_uses_the_api_when_the_download_path_fails(self, sleep):
        asset = "https://api.github.com/repos/example/tool/releases/assets/7"
        release = json.dumps(
            {
                "assets": [
                    {"name": "other.zip", "url": asset + "0"},
                    {"name": "tool.tar.gz", "url": asset},
                ]
            }
        ).encode()
        faults = [http_error(504)] * tools.DOWNLOAD_ATTEMPTS
        with (
            patch(
                "tools.urllib.request.urlopen",
                side_effect=[*faults, response(release), response(b"tool")],
            ) as urlopen,
            redirect_stdout(io.StringIO()) as output,
        ):
            self.assertEqual(tools.download(URL), b"tool")
        lookup, read = (call.args[0] for call in urlopen.call_args_list[-2:])
        self.assertEqual(
            lookup.full_url,
            "https://api.github.com/repos/example/tool/releases/tags/v1",
        )
        self.assertEqual(read.full_url, asset)
        self.assertEqual(read.get_header("Accept"), "application/octet-stream")
        self.assertEqual(
            output.getvalue(), "Tool download uses the GitHub release API\n"
        )

    def test_missing_release_asset_is_refused(self, sleep):
        faults = [http_error(504)] * tools.DOWNLOAD_ATTEMPTS
        with (
            patch(
                "tools.urllib.request.urlopen",
                side_effect=[*faults, response(b'{"assets": []}')],
            ),
            redirect_stdout(io.StringIO()),
        ):
            with self.assertRaises(Refused):
                tools.download(URL)

    def test_client_error_on_a_release_asset_does_not_use_the_api(self, sleep):
        with patch(
            "tools.urllib.request.urlopen", side_effect=http_error(404)
        ) as urlopen:
            with self.assertRaises(urllib.error.HTTPError):
                tools.download(URL)
        self.assertEqual(urlopen.call_count, 1)


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
