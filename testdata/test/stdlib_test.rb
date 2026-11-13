# rbs_inline: enabled

require "minitest/autorun"
require "find"
require "pathname"
require "etc"
require "ostruct"
require "delegate"
require "benchmark"
require "base64"
require "csv"
require "date"
require "digest"
require "json"
require "net/http"
require "optparse"
require "securerandom"
require "set"
require "stringio"
require "strscan"
require "tempfile"
require "time"
require "tmpdir"
require "uri"
require "rubygems"
require "fiddle"
require "fcntl"
require "cgi"
require "webrick"
require "zlib"

# Returns nil on MRI; void to rugaroo.
#: () -> void
def stdlib_noop = nil

module StdlibTests
  # Was testdata/run/benchmark_mid.rb. bm/bmbm print real timings, so only their Report/Job machinery is tested.
  class EscapeTest < Minitest::Test
    def test_cgi_and_form_escapes
      assert_equal "a+b%2A~%2F%C3%A9", CGI.escape("a b*~/é")
      assert_equal "a%20b%2B", CGI.escapeURIComponent("a b+")
      assert_equal "a+b*%7E%2F", URI.encode_www_form_component("a b*~/")
      assert_equal "x=a+b*%7E", URI.encode_www_form("x" => "a b*~")
    end

    def test_time_strptime
      t = Time.strptime("Jan 2024-03-05 10:11", "Jan %Y-%m-%d %H:%M") # literal text that spells a Go layout token
      assert_equal "2024-03-05 10:11", t.strftime("%F %H:%M")
      assert_equal false, t.utc?
      assert_equal true, Time.strptime("2024-03-05 10:11 UTC", "%Y-%m-%d %H:%M %Z").utc?
      assert_equal 32400, Time.strptime("2024-03-05 10:11 +0900", "%Y-%m-%d %H:%M %z").utc_offset
      assert_equal "2024-02-29 04:45:30 UTC", Time.strptime("1709181930", "%s").utc.to_s
      e = assert_raises(ArgumentError) { Time.strptime("nope", "%Y") }
      assert_equal "invalid date or strptime format - 'nope' '%Y'", e.message
    end
  end

  class BenchmarkTest < Minitest::Test
    def test_tms_fields_format_and_arithmetic
      t = Benchmark::Tms.new(1.5, 0.25, 0.1, 0.05, 2.0, "x")
      assert_equal "1.5", t.utime.to_s
      assert_equal "0.25", t.stime.to_s
      assert_equal "0.1", t.cutime.to_s
      assert_equal "0.05", t.cstime.to_s
      assert_equal "2.0", t.real.to_s
      assert_equal "1.9000000000000001", t.total.to_s
      assert_equal "x", t.label
      assert_equal "  1.500000   0.250000   1.900000 (  2.000000)\n", t.to_s
      assert_equal "  1.500000   0.250000   1.900000 (  2.000000)\n", t.format
      assert_equal " 1.500000  0.250000  1.900000 ( 2.000000) x\n", t.format("%9.6u %9.6y %9.6t %9.6r %n\n")
      assert_equal "[\"x\", 1.5, 0.25, 0.1, 0.05, 2.0]", t.to_a.inspect

      t2 = Benchmark::Tms.new(0.5, 0.05, 0.0, 0.0, 0.6, "y")
      assert_equal "[\"\", 2.0, 0.3, 0.1, 0.05, 2.6]", (t + t2).to_a.inspect
      assert_equal "[\"\", 1.0, 0.2, 0.1, 0.05, 1.4]", (t - t2).to_a.inspect
      assert_equal "[\"\", 3.0, 0.5, 0.2, 0.1, 4.0]", (t * 2).to_a.inspect
      assert_equal "[\"\", 0.75, 0.125, 0.05, 0.025, 1.0]", (t / 2).to_a.inspect
      assert_equal "[\"\", 3.0, 0.5, 0.2, 0.1, 4.0]", (t * 2.0).to_a.inspect
      assert_equal "[\"\", 0.75, 0.125, 0.05, 0.025, 1.0]", (t / 2.0).to_a.inspect

      sum = Benchmark::Tms.new
      assert_equal "0.0", sum.total.to_s
      assert_equal "", sum.label
    end

    # real/utime/stime are non-deterministic, so Benchmark.measure is checked by invariant, not value.
    def test_measure_invariants
      r = Benchmark.measure("work") { 1 + 1 }
      assert_equal true, r.is_a?(Benchmark::Tms)
      assert_equal "work", r.label
      assert_equal true, r.utime >= 0.0 && r.stime >= 0.0 && r.real >= 0.0
      assert_equal true, r.total == r.utime + r.stime + r.cutime + r.cstime
    end

    def test_report_and_job_widths
      report = Benchmark::Report.new(3)
      r1 = report.report("aa") { 1 + 1 }
      r2 = report.report("b") { 2 + 2 }
      assert_equal 2, report.list.size
      assert_equal "aa", report.list[0].label
      assert_equal "b", report.list[1].label
      assert_equal 3, report.width
      assert_equal true, r1.is_a?(Benchmark::Tms)
      assert_equal true, r2.is_a?(Benchmark::Tms)

      report2 = Benchmark::Report.new(0)
      report2.report("longlabel") { nil }
      assert_equal 9, report2.width

      job = Benchmark::Job.new(0)
      job.report("a") { nil }
      job.report("bb") { nil }
      assert_equal 2, job.width
    end

    def test_gc_start_and_constants
      GC.start
      assert_equal "      user     system      total        real\n", Benchmark::CAPTION
      assert_equal "%10.6u %10.6y %10.6t %10.6r\n", Benchmark::FORMAT
    end
  end

  # Was testdata/run/csv_mid.rb.
  class CsvTest < Minitest::Test
    # CSV.read/foreach/open on real files (decision 53), plus row_sep/skip_blanks/force_quotes.
    def test_files_and_options
      Dir.mktmpdir do |dir|
        path = File.join(dir, "a.csv")
        File.write(path, "sku,qty\na1,4\n\na2,7\n")
        assert_equal "[[\"sku\", \"qty\"], [\"a1\", \"4\"], [], [\"a2\", \"7\"]]", CSV.read(path).inspect

        rows = [] #: Array[Array[String?]]
        CSV.foreach(path) { |row| rows << row }
        assert_equal "[[\"sku\", \"qty\"], [\"a1\", \"4\"], [], [\"a2\", \"7\"]]", rows.inspect
        assert_equal "[[\"sku\", \"qty\"], [\"a1\", \"4\"], [], [\"a2\", \"7\"]]", CSV.foreach(path).to_a.inspect

        empty = File.join(dir, "empty.csv")
        File.write(empty, "")
        assert_equal "[]", CSV.read(empty).inspect

        out = File.join(dir, "out.csv")
        CSV.open(out, "w") do |csv|
          csv << ["sku", "qty"]
          csv << ["b1", 2]
          csv << ["b2", nil]
        end
        assert_equal "sku,qty\nb1,2\nb2,\n", File.read(out)
        assert_equal "[[\"sku\", \"qty\"], [\"b1\", \"2\"], [\"b2\", nil]]", CSV.read(out).inspect

        CSV.open(out, "a") { |csv| csv << ["b3", 9] }
        assert_equal "[\"b3\", \"9\"]", CSV.read(out).last.inspect

        forced = File.join(dir, "forced.csv")
        CSV.open(forced, "w", force_quotes: true) { |csv| csv << ["a", nil, "b,c"] }
        assert_equal "\"a\",\"\",\"b,c\"\n", File.read(forced)

        sep = File.join(dir, "sep.csv")
        File.write(sep, "a;b;c")
        assert_equal "[[\"a\"], [\"b\"], [\"c\"]]", CSV.read(sep, row_sep: ";").inspect

        assert_equal "[[\"a\", \"b\"], [\"c\", \"d\"]]", CSV.parse("a,b\n\n\nc,d\n", skip_blanks: true).inspect
        assert_equal "\"a\",\"\"\n", CSV.generate_line(["a", nil], force_quotes: true)
      end
    end

    # headers: true and converters: (decision 117, #36).
    def test_headers_and_converters
      src = "name,age,score\nAda,36,9.5\nGrace,,1_000\n Linus , 08 ,0x1A\n"
      t = CSV.parse(src, headers: true)
      assert_equal ["name", "age", "score"], t.headers
      assert_equal 3, t.size
      assert_equal "Ada", t[0]["name"]
      assert_equal "36", t[0][1]
      assert_nil t[1]["age"]
      assert_equal ["Ada", "Grace", " Linus "], t["name"]
      assert_equal({ "name" => "Ada", "age" => "36", "score" => "9.5" }, t[0].to_h)
      assert_equal "#<CSV::Row \"name\":\"Ada\" \"age\":\"36\" \"score\":\"9.5\">", t[0].inspect
      assert_equal "Ada,36,9.5\n", t[0].to_s
      assert_equal src, t.to_s
      assert_equal "#<CSV::Table mode:col_or_row row_count:4>\n#{src}", t.inspect
      assert_equal [["name", "age", "score"], ["Ada", "36", "9.5"], ["Grace", nil, "1_000"], [" Linus ", " 08 ", "0x1A"]], t.to_a
      assert_equal ["Ada", "Grace", " Linus "], t.map { |r| r["name"] }
      e = assert_raises(KeyError) { t[0].fetch("missing") }
      assert_equal "key not found: missing", e.message
      n = CSV.parse(src, headers: true, converters: :numeric)
      assert_equal ["Grace", nil, 1000], n[1].fields
      assert_equal [" Linus ", 8.0, 26], n[2].fields
      assert_equal 1, n.select { |r| r["age"].nil? }.size
      assert_equal [["name", "age", "score"], ["Ada", 36, 9.5], ["Grace", nil, 1000], [" Linus ", 8.0, 26]], CSV.parse(src, converters: :numeric)
      assert_equal [["a", "b"], [1.5, 2.0]], CSV.parse("a,b\n1.5,2", converters: :float)
      assert_equal [["a", "b"], [1.5, 2]], CSV.parse("a,b\n1.5,2", converters: [:integer, :float])
      assert_equal [1, 2.5, "x"], CSV.parse_line("1,2.5,x", converters: :numeric)
      assert_equal [{ "a" => "1", "b" => "2", nil => "3" }, { "a" => "4", "b" => nil }], CSV.parse("a,b\n1,2,3\n4", headers: true).map(&:to_h)
      assert_equal({ "a" => "1" }, CSV.parse("a,a\n1,2", headers: true)[0].to_h)
      assert_equal [], CSV.parse("", headers: true).headers
      Dir.mktmpdir do |dir|
        path = File.join(dir, "p.csv")
        File.write(path, "sku,qty\na1,4\na2,7\n")
        assert_equal 11, CSV.read(path, headers: true, converters: :integer).map { |r| r["qty"] }.sum
        skus = [] #: Array[untyped]
        CSV.foreach(path, headers: true) { |row| skus << row["sku"] }
        assert_equal ["a1", "a2"], skus
        sums = [] #: Array[untyped]
        CSV.foreach(path, converters: :integer) { |row| sums << row[1] }
        assert_equal ["qty", 4, 7], sums
      end
    end
  end

  # Was testdata/run/date_mid.rb.
  class DateTest < Minitest::Test
    def test_formats_round_trip
      d = Date.new(2026, 9, 28)
      assert_equal "Mon, 28 Sep 2026 00:00:00 GMT", d.httpdate
      assert_equal "2026-09-28T00:00:00+00:00", d.rfc3339
      assert_equal "R08.09.28", d.jisx0301
      assert_equal true, Date.httpdate(d.httpdate) == d
      assert_equal true, Date.rfc3339(d.rfc3339) == d
      assert_equal true, Date.jisx0301(d.jisx0301) == d
    end

    # era boundaries, and dates before Meiji fall back to ISO 8601
    def test_jisx0301_eras
      cases = [[1868, 9, 8], [1872, 12, 31], [1873, 1, 1], [1912, 7, 29], [1912, 7, 30], [1926, 12, 24], [1926, 12, 25], [1989, 1, 7], [1989, 1, 8], [2019, 4, 30], [2019, 5, 1], [1600, 1, 1]] #: Array[[Integer, Integer, Integer]]
      got = [] #: Array[String]
      cases.each do |y, m, dd|
        dt = Date.new(y, m, dd)
        got << dt.jisx0301
        got << (Date.jisx0301(dt.jisx0301) == dt).to_s
      end
      assert_equal ["1868-09-08", "true", "1872-12-31", "true", "M06.01.01", "true", "M45.07.29", "true",
                    "T01.07.30", "true", "T15.12.24", "true", "S01.12.25", "true", "S64.01.07", "true",
                    "H01.01.08", "true", "H31.04.30", "true", "R01.05.01", "true", "1600-01-01", "true"], got
    end
  end

  # Was testdata/run/digest_mid.rb.
  class DigestTest < Minitest::Test
    def test_file
      Dir.mktmpdir do |dir|
        path = File.join(dir, "digest_mid_tmp.txt")
        File.write(path, "hello world")
        d = Digest::MD5.file(path)
        assert_equal "5eb63bbbe01eeed093cb22bb8f5acdc3", d.hexdigest
        assert_equal true, d.hexdigest == Digest::MD5.hexdigest("hello world")
      end
    end

    def test_equality
      a = Digest::SHA256.new
      b = Digest::SHA256.new
      assert_equal true, a == b
      a.update("abc")
      assert_equal false, a == b
      b.update("abc")
      assert_equal true, a == b
      assert_equal false, a == "not a digest"
    end

    def test_sha2_bit_lengths
      assert_equal true, Digest::SHA2.new(256).hexdigest == Digest::SHA256.hexdigest("")
      assert_equal true, Digest::SHA2.new(384).hexdigest == Digest::SHA384.hexdigest("")
      assert_equal true, Digest::SHA2.new(512).hexdigest == Digest::SHA512.hexdigest("")
      assert_equal true, Digest::SHA2.new.hexdigest == Digest::SHA256.hexdigest("")
      assert_equal [Digest::SHA256.hexdigest("x"), 32], [Digest::SHA2.hexdigest("x"), Digest::SHA2.digest("x").bytesize]
      e = assert_raises(ArgumentError) { Digest::SHA2.new(123) }
      assert_equal "unsupported bit length: 123", e.message
      assert_equal "616263", Digest.hexencode("abc")
    end
  end

  # Was testdata/run/json_mid.rb.
  class JsonTest < Minitest::Test
    def test_nested_objects_arrays
      nested = JSON.parse(%({"a":{"b":[1,{"c":2}]},"d":[[1,2],{"e":3}]}))
      assert_equal "{\"a\" => {\"b\" => [1, {\"c\" => 2}]}, \"d\" => [[1, 2], {\"e\" => 3}]}", nested.to_s
    end

    # numbers: int, float, negative, exponent
    def test_numbers
      assert_equal 42, JSON.parse("42")
      assert_equal(-17, JSON.parse("-17"))
      assert_equal "3.14", JSON.parse("3.14").to_s
      assert_equal "-3.14", JSON.parse("-3.14").to_s
      assert_equal "10000000000.0", JSON.parse("1e10").to_s
      assert_equal "0.0015", JSON.parse("1.5e-3").to_s
      assert_equal 0, JSON.parse("0")
      assert_equal 0, JSON.parse("-0")
      assert_equal "-0.0", JSON.parse("-0.0").to_s
    end

    # strings with escapes/unicode
    def test_strings_with_escapes_unicode
      escaped = <<~'JSON'.chomp
      "a\tb\nc\"d\\e"
      JSON
      assert_equal "a\tb\nc\"d\\e", JSON.parse(escaped)
      assert_equal "café", JSON.parse('"café"')
      assert_equal "😀", JSON.parse('"😀"')
    end

    def test_symbolize_names
      assert_equal "{a: 1, nested: {b: 2}}", JSON.parse(%({"a":1,"nested":{"b":2}}), symbolize_names: true).to_s
    end

    def test_malformed_input
      {
        "{bad json" => "expected object key, got 'bad' at line 1 column 2",
        "" => "unexpected end of input at line 1 column 1",
        %({"a":1,}) => "expected object key, got: '}' at line 1 column 8",
        "{bad}" => "expected object key, got 'bad}' at line 1 column 2",
        "{  bad" => "expected object key, got 'bad' at line 1 column 4",
        "[1,]" => "unexpected character: ']' at line 1 column 4",
        "[1,,2]" => "unexpected character: ',2]' at line 1 column 4",
        "[1 2]" => "expected ',' or ']' after array value at line 1 column 4",
        %([truee]) => "expected ',' or ']' after array value at line 1 column 6",
        %({"a" 1}) => "expected ':' after object key at line 1 column 6",
        %({"a":1 "b":2}) => "expected ',' or '}' after object value, got: '\"b\":2}' at line 1 column 8",
        "nul" => "unexpected token 'nul' at line 1 column 1",
        "nulx" => "unexpected token 'nulx' at line 1 column 1",
        %({"a":nul}) => "unexpected token 'nul}' at line 1 column 6",
        "falsy" => "unexpected token 'falsy' at line 1 column 1",
        %({"a":}) => "unexpected character: '}' at line 1 column 6",
        "[" => "unexpected end of input at line 1 column 2",
        "{" => "expected object key, got EOF at line 1 column 2",
        "\"abc" => "unexpected end of input, expected closing \" at line 1 column 5",
        "1 2" => "unexpected token at end of stream '2' at line 1 column 3",
        %({"a":1}x) => "unexpected token at end of stream 'x' at line 1 column 8",
        "[1,2]  3 4" => "unexpected token at end of stream '3' at line 1 column 8",
        "[1]]" => "unexpected token at end of stream ']' at line 1 column 4",
        "[-]" => "invalid number: '-]' at line 1 column 2",
        "[1.]" => "invalid number: '1.]' at line 1 column 2",
        "\n\n  [1,\n 2,]" => "unexpected character: ']' at line 4 column 4",
      }.each do |src, msg|
        e = assert_raises(JSON::ParserError) { JSON.parse(src) }
        assert_equal msg, e.message
      end
    end

    def test_pretty_generate
      assert_equal "{\n  \"name\": \"Ada\",\n  \"langs\": [\n    \"ruby\",\n    \"go\"\n  ],\n  \"meta\": {\n    \"active\": true\n  }\n}",
                   JSON.pretty_generate({ "name" => "Ada", "langs" => ["ruby", "go"], "meta" => { "active" => true } })
    end

    def test_dump_load_round_trip
      data = { "x" => 1, "y" => [1, 2, 3] }
      dumped = JSON.dump(data)
      assert_equal "{\"x\":1,\"y\":[1,2,3]}", dumped
      loaded = JSON.load(dumped)
      assert_equal "{\"x\" => 1, \"y\" => [1, 2, 3]}", loaded.to_s
      assert_equal true, loaded == data
    end

    def test_generate_options_round_trip
      opts_out = JSON.generate({ "k" => "v" }, indent: "\t", space: " ", object_nl: "\n")
      assert_equal "{\n\t\"k\": \"v\"\n}", opts_out
      assert_equal true, JSON.parse(opts_out) == { "k" => "v" }
    end

    # generate -> parse round trip for a mixed structure
    def test_generate_parse_round_trip
      complex = { "list" => [1, 2.5, "three", true, false, nil, { "nested" => [1, 2] }] }
      round = JSON.parse(JSON.generate(complex))
      assert_equal true, round == complex
    end
  end

  # Was testdata/run/net_http_mid.rb. WEBrick on an ephemeral port, torn down in the same test.
  class NetHttpTest < Minitest::Test
    def test_client_against_webrick
      server = WEBrick::HTTPServer.new(Port: 0, BindAddress: "127.0.0.1")
      server.mount_proc("/") do |req, res|
        case req.path
        when "/ok"
          res.body = "ok"
        when "/created"
          res.status = 201
          res.body = "created"
        when "/badreq"
          raise WEBrick::HTTPStatus::BadRequest, "bad"
        when "/auth"
          if req["Authorization"] == "Basic #{Base64.strict_encode64("alice:secret")}"
            res.body = "welcome"
          else
            res.status = 401
            res.body = "nope"
          end
        when "/form"
          res.body = req.query.inspect
        when "/headers"
          pairs = []
          req.each { |k, v| pairs << "#{k}=#{v}" if k.start_with?("x-") }
          res.body = pairs.sort.join(",")
        when "/redirect"
          res.set_redirect(WEBrick::HTTPStatus::Found, "/ok")
        when "/setcookie"
          res.cookies << WEBrick::Cookie.new("sid", "abc123")
          res.body = "cookie set"
        when "/readcookie"
          res.body = req.cookies.map { |c| "#{c.name}=#{c.value}" }.join(",")
        else
          res.status = 404
          res.body = "missing"
        end
      end
      thread = Thread.new { server.start }
      port = server.config[:Port]

      http = Net::HTTP.new("127.0.0.1", port)

      # response classes: case/when dispatch, is_a? against the category
      kinds = ["/ok", "/created", "/missing", "/badreq"].map do |path|
        res = http.get(path)
        kind = case res
               when Net::HTTPOK then "OK"
               when Net::HTTPCreated then "Created"
               when Net::HTTPNotFound then "NotFound"
               when Net::HTTPBadRequest then "BadRequest"
               else "Other"
               end
        "#{res.code} #{kind} success=#{res.is_a?(Net::HTTPSuccess)}"
      end
      assert_equal ["200 OK success=true", "201 Created success=true", "404 NotFound success=false", "400 BadRequest success=false"], kinds

      # request objects: custom headers, basic auth, form data
      req = Net::HTTP::Get.new("/headers")
      req["X-Foo"] = "1"
      req["X-Bar"] = "2"
      assert_equal "x-bar=2,x-foo=1", http.request(req).body

      req2 = Net::HTTP::Get.new("/auth")
      req2.basic_auth("alice", "secret")
      res2 = http.request(req2)
      assert_equal "200", res2.code
      assert_equal "welcome", res2.body

      req3 = Net::HTTP::Get.new("/auth")
      res3 = http.request(req3)
      assert_equal "401", res3.code
      assert_equal "nope", res3.body

      form_req = Net::HTTP::Post.new("/form")
      form_req.set_form_data({ "a" => "1", "b" => "two words" })
      assert_equal "{\"a\" => \"1\", \"b\" => \"two words\"}", http.request(form_req).body

      # redirect: body/Location set before the raise, status from the HTTPStatus class
      res4 = http.get("/redirect")
      assert_equal "302", res4.code
      assert_equal true, (res4["Location"] || "").end_with?("/ok")
      assert_equal true, res4.is_a?(Net::HTTPRedirection)

      # WEBrick cookies: set on the response, read back on the request
      res5 = http.get("/setcookie")
      assert_equal "sid=abc123", res5["Set-Cookie"]

      req6 = Net::HTTP::Get.new("/readcookie")
      req6["Cookie"] = "sid=abc123; theme=dark"
      assert_equal "sid=abc123,theme=dark", http.request(req6).body

      # Net::HTTP class-level convenience over a URI
      uri = URI("http://127.0.0.1:#{port}/ok")
      assert_equal "ok", Net::HTTP.get(uri)
      assert_equal "ok", Net::HTTP.get_response(uri).body

      form_uri = URI("http://127.0.0.1:#{port}/form")
      assert_equal "{\"x\" => \"y\"}", Net::HTTP.post_form(form_uri, { "x" => "y" }).body

      started = "" #: String
      Net::HTTP.start("127.0.0.1", port) do |http2|
        started = http2.get("/ok").body
      end
      assert_equal "ok", started

      # use_ssl/open_timeout/read_timeout accessors
      http.open_timeout = 5.0
      http.read_timeout = 5.0
      assert_equal "5.0", http.open_timeout.to_s
      assert_equal "5.0", http.read_timeout.to_s
      http.use_ssl = false
      assert_equal false, http.use_ssl?

      server.shutdown
      thread.join
    end
  end

  # Was testdata/run/securerandom_mid.rb. Random values are checked by shape, not value.
  class SecureRandomTest < Minitest::Test
    def test_uuids
      assert_equal true, SecureRandom.uuid_v4.match?(/\A\h{8}-\h{4}-4\h{3}-[89ab]\h{3}-\h{12}\z/)
      v7 = SecureRandom.uuid_v7
      assert_equal true, v7.match?(/\A\h{8}-\h{4}-7\h{3}-[89ab]\h{3}-\h{12}\z/)
      v7b = SecureRandom.uuid_v7
      assert_equal true, v7 != v7b
    end

    def test_alphanumeric
      assert_equal true, SecureRandom.alphanumeric(20).match?(/\A[A-Za-z0-9]{20}\z/)
      assert_equal true, SecureRandom.alphanumeric(10, chars: ["x", "y", "z"]).match?(/\A[xyz]{10}\z/)
      assert_equal 16, SecureRandom.alphanumeric(8, chars: ["ab", "cd"]).length
    end
  end

  # Was testdata/run/set_mid.rb.
  class SetTest < Minitest::Test
    # MRI's union family takes any enumerable; an Array is what real code passes (Homebrew's `paths += array`).
    def test_set_ops_take_an_array
      s = Set[1, 2]
      paths = Set.new
      paths += ["a", "b"]
      paths << "c"
      s.merge([8])
      assert_equal [[1, 2, 8, 3], [1, 2, 8, 4], [2], [2, 8], %w[a b c]],
                   [(s | [3]).to_a, (s + [4]).to_a, (s & [2, 9]).to_a, (s - [1]).to_a, paths.to_a]
    end

    def test_constructors
      plain = Set.new([3, 1, 2])
      assert_equal [1, 2, 3], plain.sort
      from_range = Set.new(1..4) #: Set[Integer]
      assert_equal [1, 2, 3, 4], from_range.sort
      from_set = Set.new(from_range) #: Set[Integer]
      assert_equal [1, 2, 3, 4], from_set.sort
      h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      from_hash = Set.new(h) #: Set[[String, Integer]]
      assert_equal [["a", 1], ["b", 2]], from_hash.sort_by { |pair| pair[0] }
      doubled = Set.new([1, 2, 3]) { |x| x * 2 } #: Set[Integer]
      assert_equal [2, 4, 6], doubled.sort
      strs = Set.new(1..3) { |x| x.to_s } #: Set[String]
      assert_equal ["1", "2", "3"], strs.sort
    end

    def test_bang_methods
      m = Set.new([1, 2, 3])
      m.map! { |x| x * 10 }
      assert_equal [10, 20, 30], m.sort
      sel = Set.new([1, 2, 3, 4, 5, 6])
      assert_equal [2, 4, 6], sel.select! { |x| x.even? }.sort
      assert_nil(sel.select! { |x| x.even? })
      rej = Set.new([1, 2, 3, 4, 5, 6])
      assert_equal [1, 3, 5], rej.reject! { |x| x.even? }.sort
      assert_nil(rej.reject! { |x| x.even? })
    end

    def test_classify_and_divide
      grouped = Set.new([1, 2, 3, 4, 5, 6]).classify { |x| x % 3 }
      assert_equal [0, 1, 2], grouped.keys.sort
      assert_equal [3, 6], grouped[0].sort
      assert_equal [1, 4], grouped[1].sort
      assert_equal [2, 5], grouped[2].sort
      divided = Set.new([1, 2, 3, 4, 5, 6]).divide { |x| x % 3 }
      assert_equal [[1, 4], [2, 5], [3, 6]], divided.map(&:sort).sort_by(&:first)
    end

    # Array#to_set and Enumerable#to_set (#55, decision 139)
    def test_to_set
      assert_equal Set[1, 2, 3], [1, 2, 2, 3].to_set
      assert_equal [1, 2, 3, 4], (1..4).to_set.to_a
      assert_equal [[:a, 1]], { a: 1 }.to_set.to_a
      assert_equal [["a", 0], ["b", 1], ["a", 2]], %w[a b a].each_with_index.to_set.to_a
      assert_equal [[1, 2], [3, 4]], [[1, 2], [3, 4]].to_set.to_a
      s = Set[3, 1]
      assert_same s, s.to_set
      assert_equal [10, 20], [1, 2].to_set { |v| v * 10 }.to_a
      assert_equal [3], { 1 => 2 }.to_set { |k, v| k + v }.to_a
      assert_equal %w[1 2 3], (1..3).to_set(&:to_s).to_a
      assert_equal [2, 4, 6], (1..3).lazy.map { |x| x * 2 }.to_set.to_a
      assert_equal 1, [[1, 2].to_set, [2, 1].to_set].to_set.size
      assert_equal %w[x y], SetBag.new.to_set.to_a
      assert_equal true, [1].respond_to?(:to_set)
    end
  end

  class SetBag
    include Enumerable #[String]

    #: () { (String) -> void } -> void
    def each
      yield "x"
      yield "y"
      yield "x"
    end
  end

  # Was testdata/run/stringio_mid.rb.
  class StringIOTest < Minitest::Test
    # writes from several threads all land: each StringIO call holds the object's lock, as MRI's GVL makes it atomic (decision 461)
    def test_concurrent_writes
      20.times do
        io = StringIO.new
        threads = 8.times.map { |i| Thread.new { 50.times { io.write(i.to_s) } } }
        threads.each(&:join)
        assert_equal 400, io.string.size
      end
    end

    # Kernel#printf with an IO first writes there (#78's printf specs)
    def test_printf_to_io
      io = StringIO.new(+"")
      printf(io, "%d-%s|", 1, "a")
      printf(io, "%.1f", 2.25)
      assert_equal "1-a|2.2", io.string
    end

    def test_read_at_eof
      eof_io = StringIO.new("hé\nx")
      assert_equal ["hé\n", "x"], [eof_io.readline, eof_io.readchar]
      e = assert_raises(EOFError) { eof_io.readline }
      assert_equal "end of file reached", e.message
      assert_raises(EOFError) { eof_io.readchar }
      assert_raises(EOFError) { eof_io.readbyte }
      eof_io.rewind
      assert_equal [104, 195], [eof_io.readbyte, eof_io.readbyte]
      chomp_io = StringIO.new("a\r\nb\r")
      assert_equal ["a", "b\r"], [chomp_io.readline(chomp: true), chomp_io.readline(chomp: true)]
    end

    def test_seek
      seek_io = StringIO.new("hello world")
      seek_io.seek(2)
      assert_equal 2, seek_io.tell
      seek_io.seek(2, IO::SEEK_CUR)
      assert_equal 4, seek_io.tell
      seek_io.seek(-2, IO::SEEK_END)
      assert_equal 9, seek_io.tell
      assert_equal "ld", seek_io.read
      e = assert_raises(Errno::EINVAL) { seek_io.seek(-100, IO::SEEK_SET) }
      assert_equal "Invalid argument", e.message
    end

    def test_close
      closable = StringIO.new("abc")
      assert_equal false, closable.closed?
      closable.close
      assert_equal true, closable.closed?
      e = assert_raises(IOError) { closable.read }
      assert_equal "not opened for reading", e.message
      e = assert_raises(IOError) { closable.write("x") }
      assert_equal "not opened for writing", e.message
      e = assert_raises(IOError) { closable.seek(0) }
      assert_equal "closed stream", e.message

      half_closed = StringIO.new("abc", "r+")
      half_closed.close_write
      assert_equal false, half_closed.closed?
      assert_equal "abc", half_closed.read
      e = assert_raises(IOError) { half_closed.write("z") }
      assert_equal "not opened for writing", e.message
      e = assert_raises(IOError) { StringIO.new("abc", "r").close_write }
      assert_equal "closing non-duplex IO for writing", e.message
    end

    def test_chars_and_bytes
      byte_io = StringIO.new("héllo")
      chars = [] #: Array[String]
      byte_io.each_char { |c| chars << c }
      assert_equal ["h", "é", "l", "l", "o"], chars
      byte_io.rewind
      bytes = [] #: Array[Integer]
      byte_io.each_byte { |b| bytes << b }
      assert_equal [104, 195, 169, 108, 108, 111], bytes
      byte_io.rewind
      assert_equal 104, byte_io.getbyte
      assert_equal 195, byte_io.getbyte
      empty_io = StringIO.new("")
      assert_nil empty_io.getbyte
    end

    def test_ungetc
      unget_io = StringIO.new("abc")
      unget_io.getc
      unget_io.ungetc("X")
      assert_equal "Xbc", unget_io.string
      assert_equal 0, unget_io.pos
      unget_io.rewind
      unget_io.ungetc("XYZ")
      assert_equal "XYZXbc", unget_io.string
      assert_equal 0, unget_io.pos
      eof_unget = StringIO.new("abc")
      eof_unget.read
      eof_unget.ungetc("Z")
      assert_equal "abZ", eof_unget.string
      assert_equal 2, eof_unget.pos
    end

    def test_truncate
      trunc_io = StringIO.new("hello world")
      trunc_io.pos = 100
      trunc_io.truncate(3)
      assert_equal "hel", trunc_io.string
      assert_equal 100, trunc_io.pos
      grow_io = StringIO.new("hi")
      grow_io.truncate(5)
      assert_equal 5, grow_io.string.bytesize
      assert_equal [0, 0, 0], grow_io.string.bytes.last(3)
      e = assert_raises(Errno::EINVAL) { trunc_io.truncate(-1) }
      assert_equal "Invalid argument - negative length", e.message
    end

    def test_modes
      read_only = StringIO.new("abc", "r")
      e = assert_raises(IOError) { read_only.write("x") }
      assert_equal "not opened for writing", e.message
      assert_equal "abc", read_only.read

      write_only = StringIO.new("abc", "w")
      assert_equal "", write_only.string
      e = assert_raises(IOError) { write_only.read }
      assert_equal "not opened for reading", e.message
      write_only.write("XY")
      assert_equal "XY", write_only.string

      append_io = StringIO.new("abc", "a")
      append_io.pos = 0
      append_io.write("Z")
      assert_equal "abcZ", append_io.string
      assert_equal 4, append_io.pos

      e = assert_raises(ArgumentError) { StringIO.new("abc", "nope") }
      assert_equal "invalid access mode nope", e.message
    end
  end

  # Was testdata/run/strscan_mid.rb.
  class StringScannerTest < Minitest::Test
    def test_named_captures_and_values_at
      s = StringScanner.new("2024-09-28")
      s.scan(/(?<year>\d+)-(?<month>\d+)-(?<day>\d+)/)
      assert_equal "{\"year\" => \"2024\", \"month\" => \"09\", \"day\" => \"28\"}", s.named_captures.inspect
      assert_equal ["2024-09-28", "2024", "09", "28"], s.values_at(0, 1, 2, 3)
      assert_equal ["28", "09"], s.values_at(-1, -2)
      s2 = StringScanner.new("no match here")
      s2.scan(/xyz/)
      assert_equal "{}", s2.named_captures.inspect
    end

    def test_concat
      s3 = StringScanner.new("abc")
      s3 << "def"
      assert_equal "abcdefghi", s3.concat("ghi").string
      assert_equal "abcdefghi", s3.rest
    end

    # A String pattern matches itself literally (Homebrew's Locale.try_parse `scanner.scan("-")`)
    def test_scan_byte_and_peek_byte
      s = StringScanner.new("a\xC3\xA9".b)
      assert_equal 0x61, s.peek_byte
      assert_equal 0x61, s.scan_byte
      assert_equal 0xC3, s.scan_byte
      assert_equal 0xA9, s.peek_byte
      assert_equal 0xA9, s.scan_byte
      assert_nil s.peek_byte
      assert_nil s.scan_byte
      assert_equal 3, s.pos
    end

    def test_scanner_string_patterns
      s = StringScanner.new("en-US.x")
      assert_equal ["en", "-", "-", 2, 1, nil], [s.scan(/[a-z]+/), s.check("-"), s.scan("-"), s.skip("US"), s.match?("."), s.scan("x")]
    end

    def test_scan_full_and_search_full
      s4 = StringScanner.new("foo5bar")
      assert_equal "foo5bar", s4.scan_full(/\w+/, true, true)
      assert_equal 7, s4.pos
      s5 = StringScanner.new("test string")
      assert_equal "test", s5.scan_full(/\w+/, false, true)
      assert_equal 0, s5.pos
      assert_equal 4, s5.scan_full(/\w+/, false, false)
      assert_equal 0, s5.pos
      assert_nil s5.scan_full(/xyz/, true, true)
      s6 = StringScanner.new("hello world")
      assert_equal "hello wor", s6.search_full(/wor/, true, true)
      assert_equal 9, s6.pos
      s7 = StringScanner.new("hello world")
      assert_equal "hello wor", s7.search_full(/wor/, false, true)
      assert_equal 0, s7.pos
      assert_equal 9, s7.search_full(/wor/, false, false)
      assert_nil s7.search_full(/xxx/, true, true)
    end
  end

  # Was testdata/run/thread_mid.rb. Every thread is joined before its result is read, so no ordering is timing-dependent.
  class ThreadTest < Minitest::Test
    # Thread.new forwards constructor args into the block, as MRI's does.
    def test_new_forwards_args
      got = Queue.new #: Queue[String]
      sum_t = Thread.new(3, 4) { |a, b| got << (a + b).to_s }
      sum_t.join
      label_t = Thread.new("x", 2, 3) { |s, a, b| got << "#{s}#{a + b}" }
      label_t.join
      assert_equal "7", got.pop
      assert_equal "x5", got.pop
    end

    # value is the block's last value; a void call there (tap's too) gives nil.
    def test_value
      assert_equal 42, Thread.new { 6 * 7 }.value
      assert_equal "ab", Thread.new("a") { |s| s + "b" }.value
      assert_nil Thread.new { stdlib_noop }.value
      assert_equal [1, 2], [1, 2].tap { stdlib_noop }
    end

    # Homebrew's Utils.parallel_map: a quiet thread's raise still reaches value
    def test_report_on_exception_false
      t = Thread.new do
        Thread.current.report_on_exception = false
        raise "quiet"
      end
      msg = begin
        t.value
        nil
      rescue RuntimeError => e
        e.message
      end
      assert_equal ["quiet", false, true], [msg, t.report_on_exception, Thread.current.report_on_exception]
    end

    def test_join_status_and_name
      # join(timeout): a Queue with nothing pushed blocks forever, so the timeout always fires.
      blocker = Queue.new #: Queue[Integer]
      stuck = Thread.new { blocker.pop }
      assert_nil stuck.join(0.01)

      # join(timeout) on an already-finished thread always succeeds, no race with its work.
      quick = Thread.new { 1 + 1 }
      quick.join
      assert_equal true, quick.join(0.01) == quick
      assert_equal false, quick.status

      boom_t = Thread.new { raise "boom" }
      begin
        boom_t.join
      rescue RuntimeError
      end
      assert_nil boom_t.status

      named = Thread.new { 1 }
      assert_nil named.name
      named.name = "worker"
      assert_equal "worker", named.name
      named.join

      blocker.close
      stuck.join
    end

    # Goroutine identity (decision 104): Thread.current, Mutex#owned? and MRI's locking errors.
    def test_thread_identity
      assert_same Thread.main, Thread.current
      t = Thread.new { Thread.current }
      assert_same t, t.value
      assert_match(/#<Thread:0x[0-9a-f]+ \S*stdlib_test\.rb:\d+ dead>\z/, t.inspect)
      assert_match(/#<Thread:0x[0-9a-f]+ run>\z/, Thread.main.inspect)
      assert_equal false, Thread.new { Thread.current.equal?(Thread.main) }.value
      m = Mutex.new
      assert_equal false, m.owned?
      m.lock
      assert_equal true, m.owned?
      assert_equal false, Thread.new { m.owned? }.value
      e = assert_raises(ThreadError) { m.lock }
      assert_equal "deadlock; recursive locking", e.message
      e = assert_raises(ThreadError) { Thread.new { m.unlock }.value }
      assert_equal "Attempt to unlock a mutex which is locked by another thread/fiber", e.message
      assert_equal true, m.locked?
      m.unlock
      assert_equal false, m.owned?
      m.synchronize { assert_equal true, m.owned? }
      e = assert_raises(ThreadError) { Thread.new { Thread.current.join }.value }
      assert_equal "Target thread must not be current thread", e.message
      e = assert_raises(ThreadError) { Thread.main.value }
      assert_equal "Target thread must not be current thread", e.message
    end

    # ConditionVariable#wait(timeout): nobody signals gate_cv, so this always times out.
    def test_condition_variable_wait_timeout
      gate_m = Mutex.new
      gate_cv = ConditionVariable.new
      waited = "" #: String
      gate_m.synchronize { waited = gate_cv.wait(gate_m, 0.01).inspect }
      assert_equal "nil", waited
    end

    def test_queues
      seeded = Queue.new([1, 2, 3]) #: Queue[Integer]
      assert_equal 3, seeded.size
      assert_equal 1, seeded.pop
      assert_equal 2, seeded.pop
      assert_equal 3, seeded.pop

      sq = SizedQueue.new(1) #: SizedQueue[Integer]
      assert_equal 1, sq.max
      sq.max = 3
      assert_equal 3, sq.max
      sq.max = 1

      # num_waiting: spin (no sleep in this subset) until the second push actually blocks.
      sq.push(1)
      filler = Thread.new { sq.push(2) }
      until sq.num_waiting > 0
      end
      assert_equal 1, sq.num_waiting
      sq.pop
      filler.join
      assert_equal 0, sq.num_waiting
    end

    # SizedQueue is a Go channel (decision 403): max= swaps it under blocked pushers, close wakes poppers, order holds.
    def test_sized_queue_channel
      sq = SizedQueue.new(2) #: SizedQueue[Integer]
      sq << 1
      sq << 2
      pusher = Thread.new { sq.push(3) }
      until sq.num_waiting > 0
      end
      sq.max = 4
      pusher.join
      sq << 4
      assert_equal [4, 4, 0], [sq.max, sq.size, sq.num_waiting]
      sq.max = 1
      assert_equal [1, 2, 3, 4], 4.times.map { sq.pop }
      assert_equal 0, sq.size

      waiter = Thread.new { sq.pop }
      until sq.num_waiting > 0
      end
      sq.close
      assert_nil waiter.value
      err = begin
        sq.push(5)
        nil
      rescue ClosedQueueError => e
        e.class
      end
      assert_equal ClosedQueueError, err

      drained = SizedQueue.new(3) #: SizedQueue[Integer]
      drained << 7
      drained << 8
      drained.close
      assert_equal [7, 8, nil], [drained.pop, drained.pop, drained.pop]
      assert_nil SizedQueue.new(1).pop(timeout: 0.01)
      assert_raises(ThreadError) { SizedQueue.new(1).pop(true) }

      busy = SizedQueue.new(8) #: SizedQueue[Integer]
      producers = 4.times.map { |i| Thread.new { 500.times { |j| busy << (i * 500 + j) } } }
      got = [] #: Array[Integer]
      lock = Mutex.new
      consumers = 3.times.map do
        Thread.new do
          while (v = busy.pop)
            lock.synchronize { got << v }
          end
        end
      end
      producers.each(&:join)
      busy.close
      consumers.each(&:join)
      assert_equal (0...2000).to_a, got.sort
    end
  end

  # Was testdata/run/time_mid.rb.
  class TimeTest < Minitest::Test
    # Time.new's 7th arg is a zone: a "+HH:MM"/"UTC"/"Z"/military-letter String, or `in:` as a trailing Hash (decision 23/39).
    def test_zone_arg
      t = Time.new(2024, 3, 10, 1, 30, 0, "+09:00")
      assert_equal "2024-03-10 01:30:00 +0900", t.to_s
      assert_equal 32400, t.utc_offset
      assert_equal false, t.utc?
      assert_equal true, Time.new(2024, 1, 1, 0, 0, 0, "UTC").utc?
      assert_equal true, Time.new(2024, 1, 1, 0, 0, 0, "Z").utc?
      assert_equal 3600, Time.new(2024, 1, 1, 0, 0, 0, "A").utc_offset
      assert_equal true, Time.new(2024, 1, 1, 0, 0, 0, "-00:00").utc?
      assert_equal "2023-11-15 03:43:20 +0530", Time.at(1_700_000_000, in: "+05:30").to_s
      assert_equal "Time", Time.now(in: "+09:00").class.to_s
      assert_equal 32400, Time.now(in: "+09:00").utc_offset
      e = assert_raises(ArgumentError) { Time.new(2024, 1, 1, 0, 0, 0, "bogus") }
      assert_equal "\"+HH:MM\", \"-HH:MM\", \"UTC\" or \"A\"..\"I\",\"K\"..\"Z\" expected for utc_offset: bogus", e.message
      e = assert_raises(ArgumentError) { Time.new(2024, 1, 1, 0, 0, 0, "+25:00") }
      assert_equal "utc_offset out of range", e.message
    end

    def test_localtime_and_rounding
      u = Time.utc(2024, 6, 1, 12, 0, 0)
      assert_equal "2024-06-01 13:00:00 +0100", u.localtime(3600).to_s
      assert_equal 3600, u.getlocal(3600).utc_offset
      assert_equal 18000, u.getlocal("+05:00").utc_offset
      assert_equal true, u.getutc.utc?
      r = Time.at(1.5)
      assert_equal "2.0", r.round.to_f.to_s
      assert_equal "1.0", r.floor.to_f.to_s
      assert_equal "2.0", r.ceil.to_f.to_s
      assert_equal "1.5", r.round(2).to_f.to_s
      f = Time.at(1.23456)
      assert_equal "1.23", f.round(2).to_f.to_s
      assert_equal "1.23", f.floor(2).to_f.to_s
      assert_equal "1.24", f.ceil(2).to_f.to_s
      n = Time.at(-1.5)
      assert_equal "-1.0", n.round.to_f.to_s
    end

    def test_file_open_flags_perm_and_expand_path_dir
      Dir.mktmpdir do |dir|
        path = File.join(dir, "o.txt")
        File.open(path, "w", 0o600) { |f| f.write("hi") }
        File.open(path, File::WRONLY | File::APPEND) { |f| f.write("!") }
        got = begin
          File.open(path, "w", flags: File::EXCL)
          nil
        rescue SystemCallError => e
          e.class
        end
        assert_equal [0o600, "hi!", Errno::EEXIST, "hi!"], [File.stat(path).mode & 0o777, File.read(path), got, File.new(path, mode: "r").read]
      end
      assert_equal ["/x/a", "/x/b", "/abs"], [File.expand_path("a", "/x"), File.expand_path("../b", "/x/y"), File.expand_path("/abs", "/x")]
    end

    # Fiddle's constants, as Homebrew's os/mac/ffi builds signatures at load; a missing library is DLError
    def test_fiddle_constants_and_dlerror
      err = begin
        Fiddle.dlopen("/nonexistent/libstdlib_nope.dylib")
        nil
      rescue Fiddle::DLError => e
        e.class
      end
      assert_equal [[1, -4, 4, 10], 8, Fiddle::DLError], [[Fiddle::TYPE_VOIDP, Fiddle::TYPE_UINT32_T, Fiddle::TYPE_INT, Fiddle::TYPE_CONST_STRING], Fiddle::SIZEOF_VOIDP, err]
    end

    # Homebrew's LockFile: an open file's stat is the path's, inode for inode
    def test_file_stat_ino
      Dir.mktmpdir do |dir|
        path = File.join(dir, "lock")
        File.open(path, "w") do |f|
          assert_equal [true, true, true], [f.stat.ino == File.stat(path).ino, f.stat.ino.positive?, f.stat.dev == File.stat(path).dev]
          f.fcntl(Fcntl::F_SETFD, Fcntl::FD_CLOEXEC)
          assert_equal [true, true, false], [File.identical?(Pathname(path), f), File.identical?(path, path), File.identical?(path, dir)]
          assert_equal [Fcntl::FD_CLOEXEC, 0], [f.fcntl(Fcntl::F_GETFD) & Fcntl::FD_CLOEXEC, f.flock(File::LOCK_EX | File::LOCK_NB)]
        end
      end
    end

    # Homebrew's install path: Stat#ctime and the access predicates, Pathname#grpowned?, Process.groups, Errno::EBUSY
    def test_stat_access_and_groups
      Dir.mktmpdir do |dir|
        path = File.join(dir, "f")
        File.write(path, "x")
        File.chmod(0o400, path)
        st = File.stat(path)
        assert_equal [true, true, false, false], [st.ctime.to_i.positive?, st.readable?, st.executable?, st.writable? && Process.euid != 0]
        assert_equal [true, true, true], [st.uid == Process.uid, st.gid.is_a?(Integer), Pathname(path).grpowned? == File.stat(path).grpowned?]
        assert_equal [true, true], [Process.groups.is_a?(Array), Process.groups.all?(Integer)]
        File.chown(nil, nil, path)
        assert_equal [true, true], [File.ctime(path) == st.ctime, Pathname(path).ctime == st.ctime]
        assert_equal [true, true, "/a/b"], [File.absolute_path(path) == path, File.absolute_path("~x") == File.join(Dir.pwd, "~x"), File.absolute_path("b", "/a")]
        assert_equal 1, Pathname(path).chown(nil, Process.gid)
        assert_equal [Errno::EBUSY, true, true], [Errno::EBUSY, Errno::EBUSY.new.is_a?(SystemCallError), Errno::EROFS.new.is_a?(SystemCallError)]
      end
    end

    def test_file_read_length_and_gets_forms
      Dir.mktmpdir do |dir|
        path = File.join(dir, "g.txt")
        File.write(path, "ab\ncd\n\n\n\nef\ngh\n")
        got = File.open(path) do |f|
          [f.read(3), f.read(0), f.gets("d"), f.gets(""), f.gets(nil), f.gets, f.read(1)]
        end
        lim = File.open(path) { |f| [f.gets(1), f.gets(4), f.gets("x", 2)] }
        assert_equal [["ab\n", "", "cd", "ef\ngh\n", nil, nil, nil], ["a", "b\n", "cd"]], [got, lim]
      end
    end

    def test_io_open_block_and_read_length
      # autoclose: false: the pipe ends stay r's and w's. Without it the block's close and io2's finalizer would close
      # fd numbers that r and w (and whatever reuses them, like a later test's listening socket) still hold.
      r, w = IO.pipe
      v = IO.open(w.fileno, "w", autoclose: false) do |io|
        io.write("hey")
        :done
      end
      w_auto = w.autoclose?
      w.close
      io2 = IO.open(r.fileno, autoclose: false)
      got = [v, io2.read(2), io2.read(5), io2.read(1), io2.autoclose?, w_auto]
      io2.close
      r.close
      assert_equal [:done, "he", "y", nil, false, true], got
    end

    def test_time_new_from_string
      t = Time.utc(2020, 12, 24, 15, 56, 17)
      assert_equal [true, true, true], [Time.new("2020-12-24T15:56:17Z") == t, Time.new("2020-12-25 00:57:47 +09:01:30") == t, Time.new("2020-12-25 00:56:17 +0900") == t]
      assert_equal ["2021-12-25 00:00:00 +0500", "2021-12-25 00:00:00 -0100", "2021-01-01 00:00:00 +1700"],
                   [Time.new("2021-12-25 00:00:00 +05:00").to_s, Time.new("2021-12-25 00:00:00", in: "-01:00").to_s, Time.new("2021", in: "+17:00").to_s]
      assert_equal [123456789, 123000], [Time.new("2020-12-25T00:56:17.123456789876 +09:00").nsec, Time.new("2020-12-25T00:56:17.123 +09:00").usec]
      ok = begin
        Time.new("2020-12-25")
        false
      rescue ArgumentError
        true
      end
      assert ok
    end

    def test_env_tz
      old = ENV["TZ"]
      got = ["JST-9", "PST8", "CET", "America/New_York", "CST6:30", "hello-foo", ""].map do |tz|
        ENV["TZ"] = tz
        t = Time.at(0)
        [t.utc_offset, t.zone, Time.local(2000, 7, 1).utc_offset]
      end
      assert_equal [[32400, "JST", 32400], [-28800, "PST", -28800], [3600, "CET", 7200], [-18000, "EST", -14400],
                    [-23400, "CST", -23400], [0, "UTC", 0], [0, "UTC", 0]], got
    ensure
      ENV["TZ"] = old
    end

    def test_accessors_and_ctime
      assert_equal 0, Time.at(0).tv_sec
      assert_equal 0, Time.at(1, in: "UTC").tv_usec
      assert_equal 0, Time.at(1, in: "UTC").tv_nsec
      assert_equal false, Time.new(2024, 1, 1, 0, 0, 0, "UTC").dst?
      assert_equal false, Time.new(2024, 1, 1, 0, 0, 0, "UTC").isdst
      ct = Time.new(2024, 1, 1, 0, 0, 0, "+09:00")
      assert_equal "Mon Jan  1 00:00:00 2024", ct.ctime
      assert_equal "Mon Jan  1 00:00:00 2024", ct.asctime
      d = Time.new(2024, 3, 15, 10, 0, 0, "+02:00").to_date
      assert_equal "2024-03-15", d.to_s
      assert_equal "Date", d.class.to_s
    end

    # Process clocks (decision 78): only differences of the monotonic clock mean anything.
    def test_process_clocks
      clk0 = Process.clock_gettime(Process::CLOCK_MONOTONIC)
      clk1 = Process.clock_gettime(Process::CLOCK_MONOTONIC)
      assert_equal true, clk1 >= clk0
      assert_equal "Float", clk0.class.to_s
      assert_equal true, Process.clock_gettime(Process::CLOCK_REALTIME) > 1_700_000_000
      assert_equal true, Process.pid > 0
      assert_equal true, Process.clock_gettime(Process::CLOCK_PROCESS_CPUTIME_ID) >= 0
      assert_equal [0, 6], [Process::CLOCK_REALTIME, Process::CLOCK_MONOTONIC]
      e = assert_raises(SystemCallError) { Process.clock_gettime(99) }
      assert_equal "Errno::EINVAL", e.class.to_s
    end
  end

  # Was testdata/run/uri_mid.rb.
  class URITest < Minitest::Test
    def test_parse
      u = URI.parse("http://user:pass@host:8080/path?q=1#frag")
      assert_equal "URI::HTTP", u.class.to_s
      assert_equal "http", u.scheme
      assert_equal "host", u.host
      assert_equal 8080, u.port
      assert_equal "/path", u.path
      assert_equal "q=1", u.query
      assert_equal "frag", u.fragment
      assert_equal "user:pass", u.userinfo
      assert_equal "user", u.user
      assert_equal "pass", u.password
      assert_equal "http://user:pass@host:8080/path?q=1#frag", u.to_s
      assert_equal "/path?q=1", u.request_uri
      assert_equal true, u.is_a?(URI::HTTP)
      assert_equal false, u.is_a?(URI::HTTPS)
      s = URI.parse("https://example.com/a")
      assert_equal "URI::HTTPS", s.class.to_s
      assert_equal 443, s.port
      assert_equal true, s.is_a?(URI::HTTP)
      h = URI.parse("http://example.com/a")
      assert_equal 80, h.port
      assert_equal "http://example.com/a", URI.parse("http://example.com:80/a").to_s
      bare = URI("http://example.com")
      assert_equal "URI::HTTP", bare.class.to_s
      assert_equal "", bare.path
      assert_equal "/", bare.request_uri
    end

    # URI.join: trailing slash, "..", absolute path, absolute override, multiple relatives
    def test_join
      assert_equal "http://x.com/a/c", URI.join("http://x.com/a/b", "c").to_s
      assert_equal "http://x.com/a/b/c", URI.join("http://x.com/a/b/", "c").to_s
      assert_equal "http://x.com/c", URI.join("http://x.com/a/b", "../c").to_s
      assert_equal "http://x.com/c", URI.join("http://x.com/a/b", "/c").to_s
      assert_equal "http://y.com/z", URI.join("http://x.com/a/b", "http://y.com/z").to_s
      assert_equal "http://x.com/a/c", URI.join("http://x.com/a/", "b", "c").to_s
      assert_equal "http://x.com/a/c", (URI.parse("http://x.com/a/b") + "c").to_s
    end

    # URI.decode_www_form is the inverse of URI.encode_www_form
    def test_www_form
      form = { "a" => "1", "b" => "hello world", "c" => "x&y=z" } #: Hash[String, String]
      encoded = URI.encode_www_form(form)
      assert_equal "a=1&b=hello+world&c=x%26y%3Dz", encoded
      assert_equal [["a", "1"], ["b", "hello world"], ["c", "x&y=z"]], URI.decode_www_form(encoded)
      assert_equal [], URI.decode_www_form("")
      assert_equal [["a", ""], ["b", "2"]], URI.decode_www_form("a&b=2")
    end

    def test_invalid
      e = assert_raises(URI::InvalidURIError) { URI.parse("http://exa mple.com") }
      assert_equal "bad URI (is not URI?): \"http://exa mple.com\"", e.message
    end
  end

  # Was testdata/run/zlib_mid.rb.
  class ZlibTest < Minitest::Test
    DATA_TEXT = "the quick brown fox jumps over the lazy dog\n" * 20
    KNOWN = "the quick brown fox jumps over the lazy dog\n"

    def test_deflate_and_gzip_round_trip
      deflated = Zlib::Deflate.deflate(DATA_TEXT)
      assert_equal true, Zlib::Inflate.inflate(deflated) == DATA_TEXT
      assert_equal true, deflated.bytesize < DATA_TEXT.bytesize
      assert_equal true, Zlib.inflate(Zlib.deflate(DATA_TEXT)) == DATA_TEXT
      gz = Zlib.gzip(DATA_TEXT)
      assert_equal true, Zlib.gunzip(gz) == DATA_TEXT
      assert_equal 31, gz.bytes[0]
      assert_equal 139, gz.bytes[1]
    end

    def test_gzip_writer_reader
      round_trip = false
      Dir.mktmpdir do |dir|
        path = File.join(dir, "out.txt.gz")
        File.open(path, "w") do |f|
          w = Zlib::GzipWriter.new(f)
          w.write(DATA_TEXT)
          w.close
        end

        File.open(path) do |f|
          r = Zlib::GzipReader.new(f)
          round_trip = r.read == DATA_TEXT
          r.close
        end
      end
      assert_equal true, round_trip

      sio = StringIO.new
      w2 = Zlib::GzipWriter.new(sio)
      w2.write(DATA_TEXT)
      w2.finish
      assert_equal true, Zlib.gunzip(sio.string) == DATA_TEXT
      sio2 = StringIO.new(sio.string)
      r2 = Zlib::GzipReader.new(sio2)
      assert_equal true, r2.read == DATA_TEXT
    end

    # blobs MRI produced, so the Go side must read real zlib/gzip framing
    def test_mri_blobs
      deflated_literal = "\x78\xDA\x2B\xC9\x48\x55\x28\x2C\xCD\x4C\xCE\x56\x48\x2A\xCA\x2F\xCF\x53\x48\xCB\xAF\x50\xC8\x2A\xCD\x2D\x28\x56\xC8\x2F\x4B\x2D\x52\x28\x01\x4A\xE7\x24\x56\x55\x2A\xA4\xE4\xA7\x73\x01\x00\x71\x40\x10\x04"
      assert_equal true, Zlib::Inflate.inflate(deflated_literal) == KNOWN
      gzipped_literal = "\x1F\x8B\x08\x00\xF9\xA0\xBA\x6A\x00\x03\x2B\xC9\x48\x55\x28\x2C\xCD\x4C\xCE\x56\x48\x2A\xCA\x2F\xCF\x53\x48\xCB\xAF\x50\xC8\x2A\xCD\x2D\x28\x56\xC8\x2F\x4B\x2D\x52\x28\x01\x4A\xE7\x24\x56\x55\x2A\xA4\xE4\xA7\x73\x01\x00\xBF\xDE\xC3\x28\x2C\x00\x00\x00"
      assert_equal true, Zlib.gunzip(gzipped_literal) == KNOWN
    end

    def test_corrupt_input_and_levels
      assert_raises(Zlib::DataError) { Zlib::Inflate.inflate("not valid zlib data") }
      assert_raises(Zlib::GzipFile::Error) { Zlib.gunzip("not valid gzip data") }
      assert_equal [0, 1, 9, -1], [Zlib::NO_COMPRESSION, Zlib::BEST_SPEED, Zlib::BEST_COMPRESSION, Zlib::DEFAULT_COMPRESSION]
      assert_equal true, Zlib::NO_COMPRESSION < Zlib::BEST_SPEED && Zlib::BEST_SPEED < Zlib::BEST_COMPRESSION && Zlib::DEFAULT_COMPRESSION < Zlib::NO_COMPRESSION
    end
  end

  # Dir.glob's ** and {a,b}, Dir[], each_child, chdir, File.stat/mtime, blockless File.open, sleep (decision 94).
  class FileGlobTest < Minitest::Test
    #: (^() -> void) -> void
    def in_tree(check)
      Dir.mktmpdir do |root|
        Dir.chdir(root) do
          %w[a a/b a/b/c x .hid].each { |d| Dir.mkdir(d) }
          %w[a/1.rb a/b/2.rb a/b/c/3.rb x/4.txt top.rb .hid/5.rb a/.dot.rb b.txt].each { |f| File.write(f, f) }
          check.call
        end
      end
    end

    def test_glob
      in_tree(lambda do
        assert_equal ["a/1.rb", "a/b/2.rb", "a/b/c/3.rb", "top.rb"], Dir.glob("**/*.rb")
        assert_equal ["a", "a/1.rb", "a/b", "a/b/2.rb", "a/b/c", "a/b/c/3.rb", "b.txt", "top.rb", "x", "x/4.txt"], Dir.glob("**/*")
        assert_equal ["a/1.rb", "a/b", "x/4.txt"], Dir.glob("{a,x}/*")
        assert_equal ["top.rb", "b.txt"], Dir.glob("*.{rb,txt}")
        assert_equal ["x/4.txt", "a/1.rb"], Dir.glob("{x,a}/*.{txt,rb}")
        assert_equal ["a/1.rb", "a/b/2.rb", "a/b/c/3.rb"], Dir.glob("a/**/*.rb")
        assert_equal ["a/", "a/b/", "a/b/c/", "x/"], Dir.glob("**/")
        assert_equal ["a/1.rb", "a/b"], Dir.glob("a/**")
        assert_equal ["a/b"], Dir.glob("**/b")
        assert_equal ["a/b/"], Dir.glob("a/*/")
        assert_equal [], Dir.glob("nope/*")
        assert_equal ["top.rb", "x/4.txt"], Dir["*.rb", "x/*"]
      end)
    end

    def test_dir_and_file
      in_tree(lambda do
        assert_equal ["1.rb", "b"], Dir.chdir("a") { Dir.glob("*") }
        assert_equal 42, Dir.chdir("x") { 42 }
        assert_equal ["a", "b.txt", "top.rb", "x"], Dir.glob("*")
        seen = [] #: Array[String]
        Dir.each_child("a") { |c| seen << c }
        assert_equal [".dot.rb", "1.rb", "b"], seen.sort
        st = File.stat("top.rb")
        assert_equal [6, true, false, false, "100644"], [st.size, st.file?, st.directory?, st.zero?, st.mode.to_s(8)]
        assert_equal true, File.stat("a").directory?
        assert_equal true, File.mtime("top.rb") <= Time.now
        f = File.open("top.rb")
        assert_equal "top.rb", f.read
        f.close
        assert_equal ["/", ":", nil], [File::SEPARATOR, File::PATH_SEPARATOR, File::ALT_SEPARATOR]
      end)
    end

    def test_sleep
      assert sleep(0.01).is_a?(Integer) # rounded seconds: 0, or 1 under load, so only the class is compared
      assert_equal 0, sleep(0)
      assert_equal "time interval must not be negative", assert_raises(ArgumentError) { sleep(-1) }.message
    end
  end

  # FileTest forwards to File's predicates (#44).
  class FileTestTest < Minitest::Test
    def test_predicates
      Dir.mktmpdir do |dir|
        full = File.join(dir, "full")
        File.write(full, "abc")
        empty = File.join(dir, "empty")
        File.write(empty, "")
        link = File.join(dir, "link")
        File.symlink(full, link)
        gone = File.join(dir, "gone")
        assert_equal [true, true, true, false], [full, empty, dir, gone].map { |p| FileTest.exist?(p) }
        assert_equal [true, false, false], [full, dir, gone].map { |p| FileTest.file?(p) }
        assert_equal [false, true, false], [full, dir, gone].map { |p| FileTest.directory?(p) }
        assert_equal [true, false], [link, full].map { |p| FileTest.symlink?(p) }
        assert_equal [false, true, false], [full, empty, gone].map { |p| FileTest.zero?(p) }
        assert_equal [false, true], [full, empty].map { |p| FileTest.empty?(p) }
        assert_equal [3, nil, nil], [full, empty, gone].map { |p| FileTest.size?(p) }
        assert_equal [3, nil, nil], [full, empty, gone].map { |p| File.size?(p) }
        assert_equal [3, 0], [full, empty].map { |p| FileTest.size(p) }
        assert_equal [true, true, false], [FileTest.readable?(full), FileTest.writable?(full), FileTest.executable?(full)]
        File.chmod(0o755, full)
        assert_equal [true, true], [FileTest.executable?(full), FileTest.owned?(full)]
        assert_equal false, FileTest.readable?(gone)
      end
    end

    # A parameter that is an Array by default and a String from its calls
    # (ruby/spec's DirSpecs.mock_dir, #79) is a union File.join takes.
    def joined(dirs = ["mock"]) = File.join("/base", dirs)

    def test_join_flattens_arrays
      assert_equal ["/base/mock", "/base/x"], [joined, joined("x")]
      assert_equal "a/b/c/d", File.join("a", ["b", ["c"]], "d")
      parts = ["x", "y"]
      assert_equal ["x/y", ""], [File.join(*parts), File.join]
      begin
        File.join("a", [1])
        flunk "no TypeError"
      rescue TypeError => e
        assert_equal "no implicit conversion of Integer into String", e.message
      end
    end
  end

  # OptionParser (decision 101): typed blocks, MRI's help layout, parse forms and errors.
  class OptionParserTest < Minitest::Test
    #: (Hash[Symbol, untyped]) -> OptionParser
    def parser_into(seen)
      OptionParser.new do |o|
        o.banner = "Usage: tool [options] FILE"
        o.on("-v", "--[no-]verbose", "Run verbosely") { |v| seen[:verbose] = v }
        o.on("-n", "--name NAME", "Name to use") { |v| seen[:name] = v.upcase }
        o.on("-c", "--count N", Integer, "How many") { |v| seen[:count] = v + 1 }
        o.on("--ratio R", Float, "A ratio") { |v| seen[:ratio] = v * 2 }
        o.on("-l", "--list A,B", Array, "A list") { |v| seen[:list] = v.size }
        o.on("--level [LEVEL]", "Optional level") { |v| seen[:level] = v }
        o.on("--depth [N]", Integer, "Optional depth") { |v| seen[:depth] = v }
        o.separator ""
        o.separator "Specific:"
        o.on("--a-very-long-option-name-that-overflows VALUE", "Desc after overflow") { |v| seen[:long] = v }
        o.on("-x", "--extra", "Multi line", "second line") { |v| seen[:extra] = v }
        o.on_tail("-h", "--help", "Show help") { |_| seen[:help] = true }
      end
    end

    def test_help
      assert_equal <<~HELP, parser_into({}).help
        Usage: tool [options] FILE
            -v, --[no-]verbose               Run verbosely
            -n, --name NAME                  Name to use
            -c, --count N                    How many
                --ratio R                    A ratio
            -l, --list A,B                   A list
                --level [LEVEL]              Optional level
                --depth [N]                  Optional depth

        Specific:
                --a-very-long-option-name-that-overflows VALUE
                                             Desc after overflow
            -x, --extra                      Multi line
                                             second line
            -h, --help                       Show help
      HELP
    end

    def test_parse
      seen = {} #: Hash[Symbol, untyped]
      argv = %w[-v --name bob -c 3 file1 --ratio=0.5 -lx,y,z --level --no-verbose --depth 4 file2 -- -z]
      assert_equal %w[file1 file2 -z], parser_into(seen).parse!(argv)
      assert_equal %w[file1 file2 -z], argv
      assert_equal({ verbose: false, name: "BOB", count: 4, ratio: 1.0, list: 3, level: nil, depth: 4 }, seen)
      seen.clear
      assert_equal [], parser_into(seen).parse(%w[--verb --lev 3 -c 0x10 --dep -xnjoe])
      assert_equal({ verbose: true, level: "3", count: 17, depth: nil, extra: true, name: "JOE" }, seen)
    end

    def test_errors
      got = %w[-z --nope -n --count=abc --extra=1 --ratio=x --verbose=1].map do |a|
        e = assert_raises(OptionParser::ParseError) { parser_into({}).parse([a]) }
        "#{e.class}: #{e.message}"
      end
      assert_equal ["OptionParser::InvalidOption: invalid option: -z", "OptionParser::InvalidOption: invalid option: --nope",
                    "OptionParser::MissingArgument: missing argument: -n", "OptionParser::InvalidArgument: invalid argument: --count=abc",
                    "OptionParser::NeedlessArgument: needless argument: --extra=1", "OptionParser::InvalidArgument: invalid argument: --ratio=x",
                    "OptionParser::NeedlessArgument: needless argument: --verbose=1"], got
      e = assert_raises(OptionParser::AmbiguousOption) { parser_into({}).parse(%w[--l]) }
      assert_equal "ambiguous option: --l", e.message
    end

    def test_into
      h = {} #: Hash[Symbol, untyped]
      o = OptionParser.new do |x|
        x.on("-v", "--verbose")
        x.on("-c", "--count N", Integer)
        x.on("--[no-]color")
        x.on("-l [LEVEL]")
      end
      assert_equal [], o.parse(%w[-v -c 2 --no-color -l rest], into: h)
      assert_equal({ verbose: true, count: 2, color: false, l: "rest" }, h)
    end

    # Switches decided at run time, the argument-style constants, Array coercion and Switch#summarize (decision 245).
    def test_runtime_switches
      opts = {} #: Hash[Symbol, untyped]
      parser = OptionParser.new
      parser.summary_indent = "  "
      parser.base.long.delete("version")
      names = ["--json"]
      parser.on(*names, "Print JSON") { |v| opts[:json] = v }
      days = "--days"
      parser.on(days, OptionParser::REQUIRED_ARGUMENT, "How many days") { |v| opts[:days] = v }
      list = "--list"
      parser.on(list, OptionParser::REQUIRED_ARGUMENT, Array, "Some list") { |v| opts[:list] = v }
      assert_equal ["wget"], parser.parse(["--json", "--days", "3", "--list=a,b", "wget"])
      assert_equal({ json: true, days: "3", list: ["a", "b"] }, opts)
      option, = parser.make_switch(["--verbose", "Be loud"])
      lines = [] #: Array[String]
      option&.summarize({}, {}, parser.summary_width, parser.summary_width - 1, parser.summary_indent) { |l| lines << l }
      assert_equal ["--verbose", "Be loud", nil], [option&.long&.first, option&.desc&.first, option&.short&.first]
      assert_equal ["      --verbose                    Be loud"], lines
      assert_raises(OptionParser::InvalidOption) { parser.parse(["--version"]) }
    end

    #: (Array[String] names) -> Array[untyped]
    def optional_run(names)
      got = [] #: Array[untyped]
      required = OptionParser::OPTIONAL_ARGUMENT
      parser = OptionParser.new
      parser.on(*names, required) { |v| got << v }
      got << parser.parse(["--json", "wget"])
      parser.parse(["--json=x"])
      got
    end

    # A run-time OPTIONAL_ARGUMENT switch (Homebrew's `flag "--json"`): true when left out, a String (not a boxed
    # String?) to an untyped block, and only a placed " [V]" argument takes the next word.
    def test_runtime_optional_argument
      assert_equal [true, ["wget"], "x"], optional_run(["--json"])
      assert_equal [nil, ["wget"], "x"], optional_run(["--json=[V]"])
      assert_equal [nil, ["wget"], "x"], optional_run(["--json [V]"])
      assert_equal "x", optional_run(["--json"]).last.dup
    end
  end

  # OpenStruct, SimpleDelegator and DelegateClass (decision 118, #35).
  class LoudString < SimpleDelegator
    #: () -> String
    def shout = "#{__getobj__.upcase}!"
  end

  # wraps the Array its typed initialize hands to super: forwarded calls are typed (decision 284)
  class DelegNums < SimpleDelegator
    #: (*Integer) -> void
    def initialize(*xs)
      super(xs)
    end

    #: () -> Array[Integer]
    def evens = __getobj__.select(&:even?)
  end

  class DelegPt
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end

    #: () -> Integer
    def dbl = x * 2
  end

  class DelegWrapped < DelegateClass(DelegPt)
    #: () -> Integer
    def triple = x * 3
  end

  # A SimpleDelegator over a Set reached through untyped code: the call is forwarded at run time (Homebrew's
  # `dependables.values.map(&:to_a)` of Requirements)
  class DelegSet < SimpleDelegator
    #: (*String names) -> void
    def initialize(*names)
      super(Set.new(names))
    end
  end

  class DelegationTest < Minitest::Test
    def test_untyped_simple_delegator
      by_spec = { stable: DelegSet.new("a", "b"), head: DelegSet.new("b", "c") } #: Hash[Symbol, untyped]
      assert_equal %w[a b c], [].union(*by_spec.values.map(&:to_a))
      one = by_spec.fetch(:stable) #: untyped
      assert_equal [2, true], [one.size, one.include?("a")]
    end

    def test_open_struct
      o = OpenStruct.new(name: "Ada", age: 36)
      o.email = "a@x"
      assert_equal "Ada", o.name
      assert_equal "a@x", o.email
      assert_nil o.missing
      assert_equal 36, o[:age]
      assert_equal "Ada", o["name"]
      assert_equal({ name: "Ada", age: 36, email: "a@x" }, o.to_h)
      assert_equal true, o.respond_to?(:name)
      assert_equal false, o.respond_to?(:nope)
      assert_equal "#<OpenStruct name=\"Ada\", age=36, email=\"a@x\">", o.inspect
      o[:age] = 37
      assert_equal 37, o.age
      assert_equal "a@x", o.delete_field(:email)
      assert_equal "#<OpenStruct>", OpenStruct.new.inspect
      assert_equal true, o == OpenStruct.new(name: "Ada", age: 37)
      assert_equal 3, OpenStruct.new(a: { b: [1, 2, 3] }).dig(:a, :b, 2)
      pairs = [] #: Array[untyped]
      o.each_pair { |k, v| pairs << [k, v] }
      assert_equal [[:name, "Ada"], [:age, 37]], pairs
      err = begin
        o.delete_field(:zzz)
        nil
      rescue NameError => e
        e.message
      end
      assert_equal "no field 'zzz' in #<OpenStruct name=\"Ada\", age=37>", err
    end

    def test_simple_delegator
      s = LoudString.new("hi")
      assert_equal 2, s.length
      assert_equal "HI!", s.shout
      assert_equal "HI", s.upcase
      assert_equal "\"hi\"", s.inspect
      assert_equal "hi", s.to_s
      assert_equal true, s == "hi"
      assert_equal "StdlibTests::LoudString", s.class.name
      assert_equal true, s.respond_to?(:length)
      s.__setobj__("bye")
      assert_equal 3, s.length
    end

    def test_simple_delegator_typed_by_initialize
      d = DelegNums.new(1, 2, 3, 4)
      assert_equal [[2, 4], [1, 3], 4, 10], [d.evens, d.reject(&:even?), d.size, d.sum]
    end

    def test_delegate_class
      w = DelegWrapped.new(DelegPt.new(4))
      assert_equal 4, w.x
      assert_equal 8, w.dbl
      assert_equal 12, w.triple
      w.__setobj__(DelegPt.new(5))
      assert_equal 10, w.dbl
    end
  end

  # ruby/spec core/file, dir, process, signal, time and thread gaps (#49)
  class RubySpecSystemTest < Minitest::Test
    def test_file_predicates
      Dir.mktmpdir do |d|
        f = File.join(d, "a.txt")
        File.write(f, "hi")
        e = File.join(d, "e.txt")
        File.write(e, "")
        l = File.join(d, "l")
        File.symlink(f, l)
        assert_equal [true, false, false], [File.zero?(e), File.empty?(f), File.zero?(File.join(d, "nope"))]
        assert_equal [true, true, false, true, true], [File.readable?(f), File.writable?(f), File.executable?(f), File.executable?(d), File.owned?(f)]
        assert_equal ["file", "directory", "link", "characterSpecial"], [File.ftype(f), File.ftype(d), File.ftype(l), File.ftype("/dev/null")]
        assert_equal [true, true, true], [File.readlink(l) == f, File.lstat(l).symlink?, File.lstat(d).directory?]
        assert_equal true, File.realpath(f) == File.realpath(File.join(d, ".", "a.txt"))
        assert_equal [2, "100600"], [File.chmod(0o600, f, e), File.stat(f).mode.to_s(8)]
        assert_equal [0, "h"], [File.truncate(f, 1), File.read(f)]
        sub = File.join(d, "sub")
        Dir.mkdir(sub)
        assert_equal [true, false, 0, false], [Dir.empty?(sub), Dir.empty?(d), Dir.delete(sub), Dir.exist?(sub)]
        missing = false
        begin
          File.realpath(File.join(d, "nope"))
        rescue Errno::ENOENT
          missing = true
        end
        assert missing
      end
      assert_equal [["/a/b", "c.rb"], "/dev/null"], [File.split("/a/b/c.rb"), File::NULL]
      assert_equal [ENV["HOME"], Dir.pwd], [Dir.home, Dir.getwd]
    end

    def test_process_signal_time_thread
      assert_equal [true, true, true], [Process.ppid > 0, Process.uid == Process.euid, Process.gid == Process.egid]
      assert_equal true, Process.getpgrp > 0
      assert_equal [2, 0, 9], [Signal.list["INT"], Signal.list["EXIT"], Signal.list["KILL"]]
      assert_equal ["INT", "TERM", "EXIT", nil], [Signal.signame(2), Signal.signame(15), Signal.signame(0), Signal.signame(99)]
      t = Time.at(0).utc
      assert_equal [true, 0, 1970], [t.gmt?, t.gmtoff, t.getgm.year]
      assert_nil Thread.pass
    end
  end

  # ruby/spec core/env gaps (#49): ENV answers the Hash methods
  class RubySpecEnvTest < Minitest::Test
    def test_env_hash_methods
      ENV["SPECENV_A"] = "1"
      ENV.update("SPECENV_B" => "2")
      ENV.merge!("SPECENV_C" => "3")
      ENV.store("SPECENV_D", "4")
      assert_equal({ "SPECENV_A" => "1", "SPECENV_B" => "2", "SPECENV_C" => "3", "SPECENV_D" => "4" }, ENV.select { |k, _v| k.start_with?("SPECENV_") }.sort.to_h)
      assert_equal [{ "SPECENV_A" => "1" }, ["2", nil]], [ENV.slice("SPECENV_A", "SPECENV_NOPE"), ENV.values_at("SPECENV_B", "SPECENV_NOPE")]
      assert_equal [true, false, true, "SPECENV_D", ["SPECENV_A", "1"]], [ENV.key?("SPECENV_C"), ENV.has_key?("SPECENV_Z"), ENV.value?("4"), ENV.key("4"), ENV.assoc("SPECENV_A")]
      assert_equal [["4"], true, false], [ENV.filter_map { |k, v| v if k == "SPECENV_D" }, ENV.map { |k, _v| k }.include?("SPECENV_A"), ENV.empty?]
      assert_equal [%w[SPECENV_A SPECENV_B SPECENV_C SPECENV_D], false, true], [ENV.reject { |k, _v| !k.start_with?("SPECENV_") }.keys.sort, ENV.except("SPECENV_A").key?("SPECENV_A"), ENV.to_a.size == ENV.size]
      ENV.delete_if { |k, _v| k == "SPECENV_A" }
      ENV.keep_if { |k, _v| k != "SPECENV_B" }
      assert_equal [nil, nil, true, "SPECENV_D"], [ENV["SPECENV_A"], ENV["SPECENV_B"], ENV.any? { |k, _v| k == "SPECENV_C" }, ENV.invert["4"]]
      keys = [] #: Array[String]
      ENV.each_key { |k| keys << k if k.start_with?("SPECENV_") }
      assert_equal %w[SPECENV_C SPECENV_D], keys.sort
    ensure
      %w[SPECENV_A SPECENV_B SPECENV_C SPECENV_D].each { |k| ENV.delete(k) }
    end
  end

  # ruby/spec core/io gaps (#49): reading by character and byte, pos and seek
  class RubySpecIOReadTest < Minitest::Test
    def test_char_reads
      Dir.mktmpdir do |d|
        path = File.join(d, "f.txt")
        File.write(path, "hé\nline2\nend")
        File.open(path) do |f|
          assert_equal ["h", "é", 10, 4, "line2\n", 10], [f.getc, f.getc, f.getbyte, f.pos, f.readline, f.tell]
          f.ungetc("X")
          assert_equal ["X", 101], [f.readchar, f.readbyte]
          f.rewind
          assert_equal "hé", f.readline(chomp: true)
          f.rewind
          assert_equal [0, "hé\n"], [f.pos, f.gets]
          f.seek(4)
          chars = [] #: Array[String]
          f.each_char { |c| chars << c }
          assert_equal ["line2\nend", nil, true], [chars.join, f.getc, f.eof?]
          e = assert_raises(EOFError) { f.readline }
          assert_equal "end of file reached", e.message
          f.seek(0)
          bytes = [] #: Array[Integer]
          f.each_byte { |b| bytes << b }
          assert_equal 13, bytes.size
        end
      end
      assert_equal IOError, EOFError.superclass
    end
  end

  # ruby/spec core/io gaps (#49): IO's class-level file helpers
  class RubySpecIOClassTest < Minitest::Test
    def test_io_file_helpers
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        b = File.join(d, "b")
        assert_equal [4, "x\ny\n", ["x\n", "y\n"]], [IO.write(a, "x\ny\n"), IO.read(a), IO.readlines(a)]
        assert_equal [1, "z", 4, "x\ny\n"], [IO.binwrite(b, "z"), IO.binread(b), IO.copy_stream(a, b), IO.read(b)]
        lines = [] #: Array[String]
        IO.foreach(a) { |l| lines << l }
        assert_equal ["x\n", "y\n"], lines
      end
    end
  end

  # ruby/spec core/dir gaps (#49): Dir objects
  class RubySpecDirTest < Minitest::Test
    def test_dir_objects
      Dir.mktmpdir do |root|
        File.write(File.join(root, "b"), "")
        File.write(File.join(root, "a"), "")
        d = Dir.new(root)
        assert_equal [true, %w[a b], %w[. .. a b]], [d.path == root, d.children.sort, d.entries.sort]
        seen = [] #: Array[String]
        d.each { |e| seen << e }
        d.close
        assert_equal %w[. .. a b], seen.sort
        assert_equal 2, Dir.open(root) { |x| x.children.size }
        kids = [] #: Array[String]
        o = Dir.open(root)
        o.each_child { |c| kids << c }
        assert_equal [%w[a b], true, true], [kids.sort, o.to_path == root, o.inspect == "#<Dir:#{root}>"]
        missing = false
        begin
          Dir.new(File.join(root, "nope"))
        rescue Errno::ENOENT
          missing = true
        end
        assert missing
      end
    end
  end

  # ruby/spec core/time gaps (#49): subsec
  class RubySpecTimeSubsecTest < Minitest::Test
    def test_subsec
      assert_equal [Rational(1, 2), 0, Rational(1, 4)], [Time.at(1.5).subsec, Time.at(0).subsec, Time.at(1.25).subsec]
      assert_equal Rational, Time.at(1.5).subsec.class
    end
  end

  # ruby/spec core/thread and core/process gaps (#49): Thread.start/fork, Process.getrlimit
  class RubySpecThreadProcessTest < Minitest::Test
    def test_thread_start
      assert_equal [2, 10, :f], [Thread.start { 1 + 1 }.value, Thread.start(5) { |x| x * 2 }.value, Thread.fork { :f }.value]
    end

    def test_getrlimit
      cur, max = Process.getrlimit(:NOFILE)
      assert_equal [true, true], [cur <= max, Process.getrlimit("CORE") == Process.getrlimit(:CORE)]
      e = assert_raises(ArgumentError) { Process.getrlimit(:NOPE) }
      assert_equal "invalid resource name: NOPE", e.message
    end
  end

  # ruby/spec core/file gaps (#49): utime and flock
  class RubySpecFileLockTest < Minitest::Test
    def test_utime_and_flock
      Dir.mktmpdir do |d|
        f = File.join(d, "a")
        File.write(f, "x")
        t = Time.at(1_000_000_000)
        assert_equal [1, true], [File.utime(t, t, f), File.mtime(f) == t]
        shared = File.open(f) { |io| [io.flock(File::LOCK_SH), io.flock(File::LOCK_UN)] }
        assert_equal [0, 0], shared
        a = File.open(f, "r+")
        a.flock(File::LOCK_EX)
        b = File.open(f, "r+")
        assert_equal false, b.flock(File::LOCK_EX | File::LOCK_NB)
        a.close
        b.close
        nil
      end
      assert_equal [1, 2, 4, 8], [File::LOCK_SH, File::LOCK_EX, File::LOCK_NB, File::LOCK_UN]
    end
  end

  # ruby/spec core/thread gaps (#49): list
  class RubySpecThreadListTest < Minitest::Test
    def test_thread_list
      q = Queue.new #: Queue[Integer]
      t1 = Thread.new { q.pop }
      t2 = Thread.new { q.pop }
      l = Thread.list
      assert_equal [true, true, true], [l.first == Thread.main, l.include?(t1), l.index(t1).to_i < l.index(t2).to_i]
      q << 1
      q << 2
      t1.join
      t2.join
      assert_equal [false, false], [Thread.list.include?(t1), Thread.list.include?(t2)]
    end
  end

  # ThreadGroup over the Thread registry (decision 132)
  class ThreadGroupTest < Minitest::Test
    def test_add_and_list
      g = ThreadGroup.new
      q = Queue.new #: Queue[Integer]
      ts = 3.times.map { |i| Thread.new { q.pop.to_i + i } }
      assert_equal [true, true, true], ts.map { |t| g.add(t).equal?(g) }
      assert_equal [true, true, true], [g.list == ts, ts.fetch(0).group.equal?(g), Thread.main.group.equal?(ThreadGroup::Default)]
      assert_equal [true, false], [ThreadGroup::Default.list.include?(Thread.main), ThreadGroup::Default.list.include?(ts.fetch(0))]
      3.times { q << 10 }
      assert_equal [10, 11, 12], ts.map(&:value)
      assert_equal [[], true], [g.list, ts.fetch(0).group.equal?(g)]
    end

    def test_new_thread_joins_creators_group
      g = ThreadGroup.new
      q = Queue.new #: Queue[Integer]
      t = Thread.new { q.pop; Thread.new { 1 }.group.equal?(g) }
      g.add(t)
      q << 1
      assert_equal true, t.value
      assert_equal true, Thread.new { 1 }.group.equal?(ThreadGroup::Default)
      g.add(Thread.main)
      assert_equal true, Thread.new { 1 }.group.equal?(g)
    ensure
      ThreadGroup::Default.add(Thread.main)
    end

    def test_enclose
      g = ThreadGroup.new
      q = Queue.new #: Queue[Integer]
      inside = Thread.new { q.pop }
      g.add(inside)
      assert_equal [false, true, true, false], [g.enclosed?, g.enclose.equal?(g), g.enclosed?, ThreadGroup::Default.enclosed?]
      outside = Thread.new { q.pop }
      e = assert_raises(ThreadError) { g.add(outside) }
      assert_equal "can't move to the enclosed thread group", e.message
      e = assert_raises(ThreadError) { ThreadGroup::Default.add(inside) }
      assert_equal "can't move from the enclosed thread group", e.message
      q << 1 << 1
      assert_equal [1, 1], [inside.value, outside.value]
      assert_equal [[], "#<ThreadGroup:"], [ThreadGroup.new.list, ThreadGroup.new.inspect[0, 14]]
    end
  end

  # ruby/spec core/io gaps (#49): File#sync
  class RubySpecFileSyncTest < Minitest::Test
    def test_sync
      Dir.mktmpdir do |d|
        path = File.join(d, "s")
        File.open(path, "w") do |f|
          assert_equal false, f.sync
          f.sync = true
          f.write("now")
          assert_equal [true, "now"], [f.sync, File.read(path)]
        end
      end
    end
  end

  # IO.sysopen and IO.new / IO.for_fd over a raw descriptor (mspec's new_fd)
  class StdlibIOFdTest < Minitest::Test
    def test_sysopen_and_new
      Dir.mktmpdir do |dir|
        path = File.join(dir, "fd.txt")
        fd = IO.sysopen(path, "w")
        io = IO.new(fd, "w")
        same = io.fileno == fd
        io.puts "hello"
        io.close
        r = IO.for_fd(IO.sysopen(path))
        line = begin
          r.gets
        ensure
          r.close
        end
        assert_equal [true, "hello\n", 1, true], [same, line, IO.new(1).fileno, $$ == Process.pid]
      end
      assert_raises(Errno::ENOENT) { IO.sysopen("/nonexistent/rugaroo/zz") }
      e = assert_raises(Errno::EBADF) { IO.new(9999) }
      assert_equal "Bad file descriptor", e.message
    end
  end

  # ruby/spec core/io gaps (#49): IO.pipe and IO.popen
  class RubySpecPipeTest < Minitest::Test
    def test_pipe
      r, w = IO.pipe
      assert_equal true, w.sync
      w.puts "x"
      w.close
      assert_equal "x\n", r.read
    end

    def test_popen
      assert_equal "hi\n", IO.popen(["echo", "hi"]) { |io| io.read }
      assert_equal true, $?&.success?
      assert_equal "a\n", IO.popen("echo a; exit 3") { |io| io.gets }
      assert_equal 3, $?&.exitstatus
      assert_equal ["1\n", "2\n"], IO.popen("printf '1\\n2\\n'") { |io| io.readlines }
    end
  end

  # IO follow-ups (#54, decision 138): pipe and popen ends are IOs, popen writes, blockless popen.
  class IOPipeTest < Minitest::Test
    def test_pipe_is_io
      r, w = IO.pipe
      assert_equal [IO, IO, true, false, true, nil], [r.class, w.class, w.is_a?(IO), r.sync, w.sync, r.pid]
      assert_equal true, r.inspect.match?(/\A#<IO:fd \d+>\z/) # the number is the kernel's next free descriptor
      assert_equal [true, true, false, false], [r.fileno > 2, r.to_i == r.fileno, r.closed?, w.tty?]
      assert_equal false, File.open(File::NULL, "w") { |f| IO.new(f.fileno).tty? } # a character device, not a terminal (decision 401)
      w << "a" << "b"
      w.print "c"
      w.puts "d"
      w.close
      assert_equal [true, "#<IO:(closed)>", "abcd\n", true], [w.closed?, w.inspect, r.read, r.eof?]
      e = assert_raises(IOError) { w.write("x") }
      assert_equal "closed stream", e.message
      e = assert_raises(IOError) { w.fileno }
      assert_equal "closed stream", e.message
      assert_equal [nil, nil, nil], [w.close, w.close_write, r.close]
    end

    def test_pipe_sides
      r, w = IO.pipe
      e = assert_raises(IOError) { r.close_write }
      assert_equal "closing non-duplex IO for writing", e.message
      e = assert_raises(IOError) { w.close_read }
      assert_equal "closing non-duplex IO for reading", e.message
      e = assert_raises(IOError) { r.write("x") }
      assert_equal "not opened for writing", e.message
      e = assert_raises(IOError) { w.read }
      assert_equal "not opened for reading", e.message
      r.close_read
      e = assert_raises(Errno::EPIPE) { w.write("x") }
      assert_equal "Broken pipe", e.message
      w.close_write
      assert_equal [true, true], [r.closed?, w.closed?]
    end

    def test_pipe_encoding
      r, w = IO.pipe
      assert_equal [Encoding::UTF_8, nil, nil, false], [r.external_encoding, w.external_encoding, r.internal_encoding, r.binmode?]
      r.set_encoding("ISO-8859-1:UTF-8")
      w.write([0xe9, 0x0a].pack("C*"))
      w.close
      assert_equal [Encoding::ISO_8859_1, Encoding::UTF_8, "é\n"], [r.external_encoding, r.internal_encoding, r.read]
      r.close
      r, w = IO.pipe
      w.set_encoding("ISO-8859-1")
      w.write("é")
      w.binmode
      w.write("é")
      assert_equal [true, Encoding::BINARY], [w.binmode?, w.external_encoding]
      w.close
      assert_equal [233, 195, 169], r.read.bytes
      r.close
    end

    def test_pipe_chars
      r, w = IO.pipe
      w.write("héllo\nb")
      w.close
      c1 = r.getc
      c2 = r.getc
      b = r.getbyte
      c3 = r.readchar
      r.ungetc("Z")
      assert_equal ["h", "é", 108, "l", "Zo\n", 1, 98], [c1, c2, b, c3, r.gets, r.lineno, r.readbyte]
      assert_raises(EOFError) { r.readbyte }
      r.close
    end

    def test_popen_write
      assert_equal "a\nb\n", IO.popen("sort", "r+") { |io| io.write("b\na\n"); io.close_write; io.read }
      got = IO.popen(["sh", "-c", "read x; echo got $x"], "w+") do |io|
        io.puts "hi"
        io.close_write
        [io.gets, io.class, io.sync]
      end
      assert_equal ["got hi\n", IO, true], got
      io = IO.popen(["sh", "-c", "cat > /dev/null; exit 4"], "w")
      io.puts "x"
      io.close_write
      assert_equal [true, 4], [io.closed?, $?&.exitstatus]
      io = IO.popen("cat", "r+")
      fd = io.fileno
      io.close_read
      assert_equal true, io.fileno != fd # the write side's descriptor now, as MRI's
      e = assert_raises(IOError) { io.read }
      assert_equal "not opened for reading", e.message
      assert_equal false, io.closed?
      io.close_write
      assert_equal [true, true], [io.closed?, $?&.success?]
      io = IO.popen("cat >/dev/null", "wb")
      e = assert_raises(IOError) { io.gets }
      assert_equal "not opened for reading", e.message
      io.close
      e = assert_raises(ArgumentError) { IO.popen("echo hi", "x") }
      assert_equal "invalid access mode x", e.message
    end

    def test_popen_blockless
      io = IO.popen("echo hi")
      assert_equal [IO, true, true, "hi\n", true], [io.class, io.pid.is_a?(Integer), io.sync, io.gets, io.eof?]
      e = assert_raises(IOError) { io.write("y") }
      assert_equal "not opened for writing", e.message
      io.close
      assert_equal [true, "#<IO:(closed)>", true], [io.closed?, io.inspect, $?&.success?]
      assert_raises(IOError) { io.pid }
      io = IO.popen(["sh", "-c", "exit 7"])
      pid = io.pid
      assert_nil io.close
      assert_equal [7, true], [$?&.exitstatus, $?&.pid == pid]
      e = assert_raises(Errno::ENOENT) { IO.popen(["no_such_cmd_rugaroo"]) }
      assert_equal "No such file or directory - no_such_cmd_rugaroo", e.message
    end
  end

  # File.atime and File::Stat#atime (#54, decision 138).
  class FileAtimeTest < Minitest::Test
    def test_atime
      Dir.mktmpdir do |d|
        path = File.join(d, "a")
        File.write(path, "x")
        t = Time.at(1_000_000_000.5)
        File.utime(t, Time.at(2_000_000_000), path)
        assert_equal [t, t, 500_000, Time.at(2_000_000_000)], [File.atime(path), File.stat(path).atime, File.stat(path).atime.usec, File.mtime(path)]
      end
      e = assert_raises(Errno::ENOENT) { File.atime("/no/such/rugaroo") }
      assert_equal "No such file or directory @ rb_file_s_atime - /no/such/rugaroo", e.message
      e = assert_raises(Errno::ENOENT) { File.mtime("/no/such/rugaroo") }
      assert_equal "No such file or directory @ rb_file_s_mtime - /no/such/rugaroo", e.message
    end
  end

  # ruby/spec core/io gaps (#49): lineno
  class RubySpecLinenoTest < Minitest::Test
    def test_lineno
      Dir.mktmpdir do |d|
        f = File.join(d, "a")
        File.write(f, "a\nb\nc\n")
        seen = [] #: Array[Integer]
        File.open(f) do |io|
          seen << io.lineno
          io.gets
          seen << io.lineno
          io.readline
          seen << io.lineno
          io.lineno = 10
          io.gets
          seen << io.lineno
          io.rewind
          seen << io.lineno
          io.each_line { |_l| nil }
          seen << io.lineno
        end
        assert_equal [0, 1, 2, 11, 0, 3], seen
      end
    end
  end

  # Warning and Kernel#warn(uplevel:, category:) (#44, decision 129).
  class WarningTest < Minitest::Test
    def warning_helper = warn("up", uplevel: 1)

    def test_categories
      # minitest's autorun turns :deprecated on (MRI's default is false)
      assert_equal [true, true, false, false], [Warning[:deprecated], Warning[:experimental], Warning[:performance], Warning[:strict_unused_block]]
      assert_equal [:deprecated, :experimental, :performance, :strict_unused_block], Warning.categories
      e = assert_raises(ArgumentError) { Warning[:nope] }
      assert_equal "unknown category: nope", e.message
      assert_raises(ArgumentError) { Warning[:nope] = true }
      assert_raises(ArgumentError) { Warning.warn("x", category: :nope) }
      assert_raises(ArgumentError) { warn("x", category: :nope) }
    end

    def test_warn
      _, err = capture_io do
        warn
        warn []
        warn nil
        warn [1, [2, "x\n"]], 3
        warn "a\n"
        warn "e", category: :experimental
        warn "p", category: :performance
        Warning.warn("raw")
      end
      assert_equal "\n1\n2\nx\n3\na\ne\nraw", err
      _, err = capture_io { assert_nil warn("r") }
      assert_equal "r\n", err
      _, err = capture_io { assert_nil Warning.warn("w\n") }
      assert_equal "w\n", err
    end

    def test_deprecated_switch
      assert_equal false, (Warning[:deprecated] = false)
      _, err = capture_io { warn "old api", category: :deprecated }
      assert_equal "", err
      Warning[:deprecated] = true
      _, err = capture_io { warn "old api", category: :deprecated }
      assert_equal "old api\n", err
    ensure
      Warning[:deprecated] = true
    end

    def test_uplevel
      _, err = capture_io { warn "here", uplevel: 0 }
      line = __LINE__ - 1
      assert_equal "#{line}: warning: here\n", err.sub(/\A[^:]*:/, "")
      _, err = capture_io { warning_helper }
      line = __LINE__ - 1
      assert_equal "#{line}: warning: up\n", err.sub(/\A[^:]*:/, "")
      _, err = capture_io { warn "far", uplevel: 1000 }
      assert_equal "warning: far\n", err
      e = assert_raises(ArgumentError) { warn "x", uplevel: -1 }
      assert_equal "negative level (-1)", e.message
    end
  end

  class UriParserTest < Minitest::Test
    def test_rfc2396_escape_unescape
      assert_equal "a%20b/c?d=%C3%A9&x[1]", URI::RFC2396_PARSER.escape("a b/c?d=é&x[1]")
      assert_equal "a b+c/d%zz✓", URI::RFC2396_PARSER.unescape("a%20b+c%2Fd%zz%E2%9C%93")
      assert_equal URI::RFC3986_Parser, URI::DEFAULT_PARSER.class
    end
  end

  class RackStdlibTest < Minitest::Test
    def test_decode_www_form_component_encoding
      assert_equal "a b&", URI.decode_www_form_component("a+b%26", Encoding::UTF_8)
    end

    def test_tempfile_prefix_and_suffix
      t = Tempfile.new(["rack", ".txt"])
      path = t.path.to_s
      assert_equal [true, true], [File.basename(path).start_with?("rack"), path.end_with?(".txt")]
      t.close
      t.unlink
    end

    # File.fnmatch as MRI's fnmatch_helper: Homebrew's Keg#link_dir globs with FNM_PATHNAME
    def test_fnmatch
      cases = [
        ["cat", "cat", 0], ["cat", "category", 0], ["c{at,ub}s", "cats", 0], ["c{at,ub}s", "cats", File::FNM_EXTGLOB],
        ["c?t", "cat", 0], ["c??t", "cat", 0], ["c*", "cats", 0], ["c*t", "c/a/b/t", 0], ["ca[a-z]", "cat", 0],
        ["ca[^t]", "cat", 0], ["cat", "CAT", 0], ["cat", "CAT", File::FNM_CASEFOLD], ["?", "/", File::FNM_PATHNAME],
        ["*", "/", File::FNM_PATHNAME], ["[/]", "/", File::FNM_PATHNAME], ["\\?", "?", 0], ["\\a", "a", 0],
        ["\\a", "\\a", File::FNM_NOESCAPE], ["[\\?]", "?", 0], ["*", ".profile", 0], ["*", ".profile", File::FNM_DOTMATCH],
        [".*", ".profile", 0], ["**/*.rb", "main.rb", File::FNM_PATHNAME], ["**/*.rb", "./main.rb", File::FNM_PATHNAME],
        ["**/*.rb", "lib/song.rb", File::FNM_PATHNAME], ["**.rb", "main.rb", File::FNM_PATHNAME], ["**.rb", "lib/song.rb", 0],
        ["*", "dave/.profile", 0], ["**/foo", "a/b/c/foo", File::FNM_PATHNAME], ["**/foo", "/a/b/c/foo", File::FNM_PATHNAME],
        ["**/foo", "c:/a/b/c/foo", File::FNM_PATHNAME], ["**/foo", "a/.b/c/foo", File::FNM_PATHNAME],
        ["**/foo", "a/.b/c/foo", File::FNM_PATHNAME | File::FNM_DOTMATCH],
        ["/opt/homebrew/opt/*/bin/wget", "/opt/homebrew/opt/wget/bin/wget", File::FNM_PATHNAME],
        ["/opt/homebrew/opt/*/bin/wget", "/opt/homebrew/Cellar/wget/1/bin/wget", File::FNM_PATHNAME],
        ["ca[", "ca[", 0], ["[a-c]x", "bx", 0], ["[!a-c]x", "bx", 0],
      ]
      assert_equal [true, false, false, true, true, false, true, true, true, false, false, true, false, false, false, true, true, true, true, false, true, true, true, false, true, true, true, true, true, true, true, false, true, true, false, false, true, false], cases.map { |pat, s, f| File.fnmatch(pat, s, f) }
      assert_equal [true, true], [Pathname.new("/a/b").fnmatch?("/a/*", File::FNM_PATHNAME), File.fnmatch?("*.rb", "x.rb")]
    end

    # stars backtrack to the last one only: k stars used to cost O(n^k), and `**/` segments compounded the same way
    def test_fnmatch_many_stars_is_linear
      many = "*a" * 14 + "*b"
      deep = "**/" * 16 + "x"
      segs = "a/" * 40
      assert_equal [false, true, false, true], [
        File.fnmatch(many, "a" * 40), File.fnmatch(many, "#{"a" * 40}b"),
        File.fnmatch(many, "a" * 40, File::FNM_PATHNAME), File.fnmatch(many, "#{"a" * 40}b", File::FNM_PATHNAME),
      ]
      assert_equal [false, true, false, false, true], [
        File.fnmatch(deep, "#{segs}y", File::FNM_PATHNAME), File.fnmatch(deep, "#{segs}x", File::FNM_PATHNAME),
        File.fnmatch(deep, "#{segs}.b/x", File::FNM_PATHNAME), File.fnmatch("a*a*b", "a/a/b", File::FNM_PATHNAME),
        File.fnmatch(deep, "#{segs}.b/x", File::FNM_PATHNAME | File::FNM_DOTMATCH),
      ]
    end

    # Homebrew's Cleanup and Keg: each_child, descend, world_writable?, glob with a block, Errno::ELOOP
    def test_pathname_walk_helpers
      Dir.mktmpdir do |d|
        r = Pathname.new(d)
        (r + "a").mkpath
        (r + "b").write("x")
        File.chmod(0o666, (r + "b").to_s)
        kids = [] #: Array[String]
        r.each_child { |c| kids << c.basename.to_s }
        down = [] #: Array[String]
        Pathname.new("/usr/local/bin").descend { |x| down << x.to_s }
        hits = [] #: Array[String]
        r.glob("*") { |x| hits << x.basename.to_s }
        Pathname.glob("#{d}/*") { |x| hits << x.basename.to_s }
        File.symlink("loop", (r + "loop").to_s)
        err = begin
          File.open((r + "loop").to_s)
          nil
        rescue Errno::ELOOP => e
          e.class
        end
        assert_equal [%w[a b], 0o666, nil, ["/", "/usr", "/usr/local", "/usr/local/bin"], %w[a a b b], Errno::ELOOP],
                     [kids.sort, (r + "b").world_writable?, (r + "a").world_writable?, down, hits.sort, err]
      end
    end

    # Homebrew's Keg#link_dir: Find.prune inside Pathname#find leaves the block and skips the directory
    def test_pathname_find_prune
      Dir.mktmpdir do |d|
        root = Pathname.new(d)
        (root + "a/skip").mkpath
        (root + "a/keep").mkpath
        (root + "a/skip/x").write("1")
        (root + "a/keep/y").write("2")
        seen = [] #: Array[String]
        root.find do |pth|
          Find.prune if pth.basename.to_s == "skip"
          seen << pth.relative_path_from(root).to_s
        end
        assert_equal [".", "a", "a/keep", "a/keep/y"], seen
      end
    end

    # Homebrew's File.atomic_write: Tempfile.open in a given directory, closed after the block, its value returned
    # blockless descend/ascend are Enumerators, and a relative path's ascend stops at its first segment (Homebrew's
    # Relocated#resolve_target `target.descend.first`)
    def test_pathname_descend_ascend_enum
      t = Pathname("~/bin/x")
      assert_equal ["~", %w[~ ~/bin ~/bin/x], %w[~/bin/x ~/bin ~]], [t.descend.first.to_s, t.descend.map(&:to_s), t.ascend.to_a.map(&:to_s)]
      assert_equal [%w[./a .], %w[. ./a ./a/b]], [Pathname("./a").ascend.to_a.map(&:to_s), Pathname("./a/b").descend.map(&:to_s)]
    end

    def test_tempfile_to_io
      t = Tempfile.new("to-io")
      io = t.to_io
      io.write("hi")
      io.flush
      assert_equal [true, "hi"], [io.is_a?(File), File.read(t.path.to_s)]
      t.close
      t.unlink
    end

    def test_tempfile_open_in_dir
      Dir.mktmpdir do |d|
        path = nil #: String?
        got = Tempfile.open(".tab", d) do |t|
          t.binmode
          t.write("x")
          path = t.path
          :done
        end
        p = path.to_s
        assert_equal [:done, true, true, "x"], [got, File.dirname(p) == d, File.basename(p).start_with?(".tab"), File.read(p)]
      end
    end
  end

  # A Pathname is an ordinary object (decision 289): a subclass, and a module with state included into it.
  module StdlibMagicShim
    # @rbs @magic: String?

    #: (untyped) -> void
    def initialize(path)
      @magic = nil
      super
    end

    #: () -> String
    def magic = @magic ||= read(2).to_s
  end

  class StdlibMagicPath < Pathname
    include StdlibMagicShim

    #: () -> String
    def stem = File.basename(self, ".txt")
  end

  # Pathname#open, Dir.mktmpdir without a block or with a prefix, IO#write of several values.
  class StdlibPathOpenTest < Minitest::Test
    def test_pathname_subclass_with_state
      Dir.mktmpdir do |d|
        File.write(File.join(d, "m.txt"), "MZ rest")
        x = StdlibMagicPath.new(File.join(d, "m.txt"))
        assert_equal ["MZ", "MZ", "m", true, "m.txt"], [x.magic, x.magic, x.stem, x.is_a?(Pathname), x.basename.to_s]
      end
    end

    def test_pathname_class_glob
      Dir.mktmpdir do |d|
        File.write(File.join(d, "a.txt"), "")
        File.write(File.join(d, "b.txt"), "")
        assert_equal %w[a.txt b.txt], Pathname.glob("#{d}/*.txt").map { |x| x.basename.to_s }.sort
        assert_equal ["a.txt"], Pathname.glob(Pathname(d) / "a*").map { |x| x.basename.to_s }
        assert_equal [2, 0], [Pathname.glob(["#{d}/a*", "#{d}/b*"]).size, Pathname.glob("#{d}/zz*").size]
      end
    end

    def test_pathname_open_and_mktmpdir
      d = Pathname(Dir.mktmpdir("stdlibt"))
      assert File.basename(d.to_s).start_with?("stdlibt")
      n = (d + "a.txt").open("w") { |f| f.write("hi\n", "there") }
      got = [n, (d + "a.txt").open("r") { |f| f.gets }, (d + "a.txt").open { |f| f.read.size }]
      assert_equal [8, "hi\n", 8], got
      assert_equal "hi\nthere", (d + "a.txt").read(encoding: Encoding::UTF_8)
      inner = Dir.mktmpdir(%w[a .z], d.to_s) { |x| [File.dirname(x) == d.to_s, x.end_with?(".z")] }
      assert_equal [true, true], inner
      FileUtils.rm_rf(d.to_s)
      refute File.exist?(d.to_s)
    end

    def test_stringio_write_several
      s = StringIO.new
      assert_equal [3, "a1b"], [s.write("a", 1, :b), s.string]
    end
  end

  # Non-blocking pipe reads, IO.select, and Process.spawn's general form with detach (decision 233).
  class StdlibSpawnTest < Minitest::Test
    def test_pipe_nonblock
      r, w = IO.pipe
      got = []
      begin
        r.read_nonblock(10)
      rescue IO::WaitReadable => e
        got << e.class.name
      end
      got << IO.select([r], nil, nil, 0.05).inspect
      w.write("ab\ncd")
      got << IO.select([r])[0].size
      got << r.read_nonblock(3) << r.readpartial(10)
      w.close
      begin
        r.read_nonblock(5)
      rescue EOFError => e2
        got << e2.class.name
      end
      assert_equal ["IO::EAGAINWaitReadable", "nil", 1, "ab\n", "cd", "EOFError"], got
    end

    def test_spawn_env_argv0_and_redirects
      out_r, out_w = IO.pipe
      pid = Process.spawn({ "SPAWN_T" => "v" }, ["/bin/sh", "sh"], "-c", "echo $SPAWN_T; exit 3", out: out_w)
      out_w.close
      status = Process.detach(pid).value
      assert_equal ["v\n", 3], [out_r.read, status.exitstatus]
    end

    # Process.detach's thread answers pid; IO.popen takes an env Hash leading the Array and spawn's options (decision 241).
    def test_detach_pid_and_popen_options
      pid = Process.spawn("sh", "-c", "exit 2")
      t = Process.detach(pid)
      assert_equal [true, true, 2], [t.pid == pid, t.is_a?(Thread), t.value.exitstatus]
      opts = { err: File::NULL, chdir: "/" } #: Hash[Symbol, untyped]
      got = IO.popen([{ "POPEN_T" => "v" }, "sh", "-c", "echo $POPEN_T; pwd; echo e >&2"], "r", opts) { |io| io.read }
      assert_equal "v\n/\n", got
      assert_equal "/\n", IO.popen(["pwd", { chdir: "/" }]) { |io| io.read }
    end

    def test_env_fetch_nil_default
      assert_equal [nil, "d", false], [ENV.fetch("RUGAROO_NO_SUCH_VAR", nil), ENV.fetch("RUGAROO_NO_SUCH_VAR", "d"), ENV.fetch("RUGAROO_NO_SUCH_VAR", false)]
      assert_raises(KeyError) { ENV.fetch("RUGAROO_NO_SUCH_VAR") }
    end

    # Gem::Version, as Homebrew's gem setup checks RubyGems' own
    def test_gem_version
      v = ->(s) { Gem::Version.new(s) }
      assert_equal [0, true, true, "1.2"], [v.("1.2.0") <=> v.("1.2"), v.("1.2.a") < v.("1.2"), v.("1.10") > v.("1.9"), v.(" 1.2 ").to_s]
      assert_equal [[1, 2, "b", 3], [1, 2], false], [v.("1.2.b3").segments, v.("1.2.0").canonical_segments, v.(Gem::VERSION) < v.("2.2.0")]
      assert_equal [true, true], [Gem.paths.home.is_a?(String), Gem.paths.path.is_a?(Array)]
    end

    def test_time_to_r_and_uname
      assert_equal [Rational(1_700_000_000, 1), true], [Time.at(1_700_000_000).to_r, Etc.uname[:sysname].size.positive?]
    end
  end

  # #102: Time.at's subsec and in:, Time.new(in:), nil zone for a fixed offset, exact Float seconds
  class StdlibTimeSubsecTest < Minitest::Test
    def test_at_subsec
      t = Time.at(100, 500500)
      assert_equal [500500, 500500000, Rational(1001, 2000)], [t.usec, t.nsec, t.subsec]
      assert_equal [3750, 9900, 5_000_000], [Time.at(100, 3.75).nsec, Time.at(100, Rational(99, 10)).nsec, Time.at(0, 5, :millisecond).nsec]
      assert_equal [1, 1, false, 100.0001], [Time.at(100, 1) <=> Time.at(100, 0), Time.at(100, 0.5) <=> Time.at(100, 0), Time.at(100, 5).eql?(Time.at(100, 6)), Time.at(100, 100).to_f]
      assert_equal TypeError, (begin; Time.at(0, nil); rescue TypeError => e; e.class; end)
    end

    def test_in_keyword_and_zone
      assert_equal [18000, 3600, -10800, nil, nil, "UTC"], [Time.at(0, in: 18000).utc_offset, Time.at(0, in: "A").utc_offset, Time.new(2000, 1, 1, in: "-03:00").utc_offset, Time.at(0, in: "+09:00").zone, Time.new(2000, 1, 1, 0, 0, 0, "+05:30").zone, Time.at(0, in: "UTC").zone]
    end

    def test_float_seconds_exact
      assert_equal [699999, 98, 899999999], [(Time.at(100) + -1.3).usec, (Time.at(100) - 1.3).to_i, Time.at(-1.1).nsec]
    end

    def test_out_of_range_messages
      msgs = [[2008, 1, 0, 0, 0, 0], [2008, 1, 1, -1, 0, 0], [2008, 1, 1, 25, 0, 0], [2008, 1, 1, 0, 60, 0], [2008, 1, 1, 0, 0, -1], [2008, 1, 1, 0, 0, 61], [2008, 1, 1, 0, 0, 64]].map do |a|
        begin
          Time.utc(a.fetch(0), a.fetch(1), a.fetch(2), a.fetch(3), a.fetch(4), a.fetch(5))
          ""
        rescue ArgumentError => e
          e.message
        end
      end
      assert_equal ["mday out of range", "argument out of range", "hour out of range", "min out of range", "argument out of range", "sec out of range", "argument out of range"], msgs
    end
  end
end

module StdlibTests
  class ScanNamedGroupTest < Minitest::Test
    class Scan < ::StringScanner
      attr_reader :start #: Integer

      #: (String, ?Integer?) -> void
      def initialize(str, start = 1)
        super(str)
        @start = start || 1
      end
    end

    def test_named_groups
      s = StringScanner.new("## Title\n")
      s.scan(/(?<level>#+) (?<contents>.*)/)
      assert_equal "##", s["level"]
      assert_equal "Title", s[:contents]
      assert_equal "##", s[1]
      assert_nil s[5]
      err = begin
        s["nope"]
        nil
      rescue IndexError => e
        e
      end
      assert_kind_of IndexError, err
    end

    def test_subclass_super
      sc = Scan.new("abc", 3)
      assert_equal "abc", sc.rest
      assert_equal 3, sc.start
      refute sc.eos?
      assert_equal 1, Scan.new("x", nil).start
    end
  end
end

module StdlibTests
  # gets(sep) and gets(nil) read through buffer-sized chunks (bufio is 4096 bytes); separators fall on, before and after that edge.
  class GetsSeparatorChunksTest < Minitest::Test
    #: (String, String?) -> Array[Integer]
    def gets_sizes(path, sep)
      File.open(path) do |f|
        out = [] #: Array[Integer]
        while (l = f.gets(sep))
          out << l.size
        end
        out << f.lineno
        out
      end
    end

    #: (String, String?, Integer) -> Array[Integer]
    def gets_limited(path, sep, limit)
      File.open(path) do |f|
        out = [] #: Array[Integer]
        while (l = f.gets(sep, limit))
          out << l.size
        end
        out << f.lineno
        out
      end
    end

    def test_gets_separators_across_buffer_boundaries
      Dir.mktmpdir do |dir|
        path = File.join(dir, "g.txt")
        File.write(path, ("x" * 4095) + "\n\n" + ("y" * 9000) + "\n\n\n" + "zab" + ("ab" * 3000) + "b" + "abb" + ("é" * 2500) + "\n\nend")
        got = ["\n\n", "", "ab", "bb", "aab", nil, "z", "é", "\n"].map { |sep| gets_sizes(path, sep) }
        assert_equal [[4097, 9002, 8510, 3, 4], [4097, 9002, 8509, 3, 4], [13103, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 3, 2506, 3003], [19104, 3, 2505, 3], [21612, 1], [21612, 1], [13101, 8511, 2], [19108, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 5, 2501], [4096, 1, 9001, 1, 1, 8508, 1, 3, 8]], got
        lim = [["\n\n", 100], ["\n\n", 5000], ["ab", 7], [nil, 5000], ["é", 3]].map { |sep, n| gets_limited(path, sep, n) }
        assert_equal [[100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 97, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 2, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 54, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 6, 3, 4], [4097, 5000, 4002, 5000, 3004, 506, 3, 4], [7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 6, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 6, 3002], [5000, 5000, 5000, 4554, 2058, 1], [3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 3, 2, 2501]], lim
      end
    end
  end

  class BacktraceCaptureTest < Minitest::Test
    #: (Integer) -> Integer
    def deep(n) = n.zero? ? raise("deep") : deep(n - 1) + 1

    def test_backtrace_depth_and_reraise
      sizes = [] #: Array[Integer]
      3.times do |i|
        deep((i * 2) + 1)
      rescue RuntimeError => e
        sizes << (e.backtrace&.size || 0)
      end
      assert_equal [2, 2], [sizes.fetch(1) - sizes.fetch(0), sizes.fetch(2) - sizes.fetch(1)]
      first = nil #: Array[String]?
      begin
        begin
          deep(1)
        rescue RuntimeError => e
          first = e.backtrace
          raise e
        end
      rescue RuntimeError => e2
        assert_equal true, first == e2.backtrace
      end
    end
  end
end

module StdlibTests
  class TimeRandomCoreGapsTest < Minitest::Test
    def test_utc_usec
      assert_equal [123456, 123456699, 500], [Time.utc(2000, 1, 1, 0, 0, 0, 123456).usec, Time.gm(2000, 1, 1, 0, 0, 0, 123456.7).nsec, Time.utc(2000, 1, 1, 0, 0, 0, Rational(1, 2)).nsec]
      assert_equal "2000-01-02 03:04:05.000006 UTC", Time.utc(2000, 1, 2, 3, 4, 5, 6).strftime("%Y-%m-%d %H:%M:%S.%6N %Z")
    end

    def test_time_deconstruct_keys
      t = Time.utc(2000, 1, 2, 3, 4, 5, Rational(1, 2))
      assert_equal({ year: 2000, month: 1, day: 2, yday: 2, wday: 0, hour: 3, min: 4, sec: 5, subsec: Rational(1, 2000000), dst: false, zone: "UTC" }, t.deconstruct_keys(nil))
      assert_equal({ year: 2000, zone: "UTC" }, t.deconstruct_keys([:year, :zone, :foo]))
      assert_equal({ zone: "UTC", year: 2000 }, t.deconstruct_keys([:zone, :year]))
    end

    def test_random_srand
      old = Random.srand(5)
      assert_equal 5, Random.srand(7)
      assert_equal 7, Random.srand(old)
    end

    # A program that seeds keeps MRI's sequence on every thread (decision 403).
    def test_srand_sequence_across_threads
      srand(42)
      got = [rand(1000)]
      Thread.new { got.push(rand(1000), rand(1..6), Random.rand(50)) }.join
      srand(42)
      assert_equal got, [rand(1000), rand(1000), rand(1..6), Random.rand(50)]
    end
  end
end
