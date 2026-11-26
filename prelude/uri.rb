# prelude/uri.rb
# rbs_inline: enabled

require_relative "ipaddr"
#
# URI.parse/join and the www-form corner (URI.encode_www_form and friends).

module URI
  class Error < StandardError; end
  class InvalidURIError < Error; end

  class Generic < Object
    #: (String?, String?, String?, Integer?, String, String?, String?) -> void
    def initialize(scheme, userinfo, host, port, path, query, fragment)
      @scheme = scheme
      @userinfo = userinfo
      @host = host
      @port = port
      @path = path
      @query = query
      @fragment = fragment
    end

    attr_reader :scheme #: String?
    attr_reader :host #: String?
    attr_reader :port #: Integer?
    attr_reader :path #: String
    attr_reader :query #: String?
    attr_reader :fragment #: String?
    attr_reader :userinfo #: String?

    #: () -> String?
    def user
      ui = @userinfo
      return nil unless ui

      ui.split(":", 2)[0]
    end

    #: () -> String?
    def password
      ui = @userinfo
      return nil unless ui

      parts = ui.split(":", 2)
      return nil if parts.length < 2

      parts[1]
    end

    #: () -> String
    def request_uri
      p = @path.empty? ? "/" : @path
      q = @query
      q ? "#{p}?#{q}" : p
    end

    #: () -> String
    def to_s = %x{
      scheme, hasScheme := rbOptStr(self.Scheme())
      userinfo, hasUserinfo := rbOptStr(self.Userinfo())
      host, hasHost := rbOptStr(self.Host())
      port, hasPort := rbOptInt(rbOptToPtr(self.Port()))
      query, hasQuery := rbOptStr(self.Query())
      fragment, hasFragment := rbOptStr(self.Fragment())
      return String(rbURIString(hasScheme, scheme, hasUserinfo, userinfo, hasHost, host, hasPort, port, string(self.Path()), hasQuery, query, hasFragment, fragment))
    }

    #: (String) -> Generic
    def merge(rel) = %x{
      scheme, hasScheme := rbOptStr(self.Scheme())
      userinfo, hasUserinfo := rbOptStr(self.Userinfo())
      host, hasHost := rbOptStr(self.Host())
      port, hasPort := rbOptInt(rbOptToPtr(self.Port()))
      query, hasQuery := rbOptStr(self.Query())
      fragment, hasFragment := rbOptStr(self.Fragment())
      base := rbURIString(hasScheme, scheme, hasUserinfo, userinfo, hasHost, host, hasPort, port, string(self.Path()), hasQuery, query, hasFragment, fragment)
      raw, err := rbURIMerge(base, string(rel))
      if err != nil {
        panic(NewURI_InvalidURIError(Ref(String(err.Error()))))
      }
      return rbURIFromRaw(raw)
    }

    #: (String) -> Generic
    def +(rel) = merge(rel)

    #: () -> String
    def inspect = "#<#{__class_name} #{to_s}>"
  end

  class HTTP < Generic; end
  class HTTPS < HTTP; end

  # The escape/unescape half of MRI's URI parsers, which Rack::Utils reaches through URI::RFC2396_PARSER.
  class RFC2396_Parser
    # With schemes, only a URI of one of them, whatever its case (MRI's lookahead in front of the same pattern).
    #: (?Array[String]? schemes) -> Regexp
    def make_regexp(schemes = nil)
      return __make_regexp("") if schemes.nil?

      __make_regexp("(?=(?i:#{schemes.map { |s| Regexp.escape(s) }.join("|")}):)")
    end

    #: (String) -> Regexp
    def __make_regexp(lead) = %x{ return rbRegexpLenient("(?m)", string(lead)+rbURIAbsRefSrc, "x") }

    #: (String) -> String
    def escape(str) = %x{ String(rbURIEscape(string(str))) }

    #: (String) -> String
    def unescape(str) = %x{ String(rbURIUnescape(string(str))) }
  end

  class RFC3986_Parser
    # With schemes, only a URI of one of them, whatever its case (MRI's lookahead in front of the same pattern).
    #: (?Array[String]? schemes) -> Regexp
    def make_regexp(schemes = nil)
      return __make_regexp("") if schemes.nil?

      __make_regexp("(?=(?i:#{schemes.map { |s| Regexp.escape(s) }.join("|")}):)")
    end

    #: (String) -> Regexp
    def __make_regexp(lead) = %x{ return rbRegexpLenient("(?m)", string(lead)+rbURIAbsRefSrc, "x") }

    #: (String) -> String
    def escape(str) = %x{ String(rbURIEscape(string(str))) }

    #: (String) -> String
    def unescape(str) = %x{ String(rbURIUnescape(string(str))) }
  end

  RFC2396_PARSER = RFC2396_Parser.new
  RFC3986_PARSER = RFC3986_Parser.new
  DEFAULT_PARSER = RFC3986_PARSER

  #: (String) -> Generic
  def self.parse(str) = %x{
    raw, err := rbURIParse(string(str))
    if err != nil {
      panic(NewURI_InvalidURIError(Ref(String(err.Error()))))
    }
    return rbURIFromRaw(raw)
  }

  #: (String, *String) -> Generic
  def self.join(base, *rest)
    result = parse(base)
    rest.each { |s| result = result.merge(s) }
    result
  end

  #: (Hash[String, String]) -> String
  def self.encode_www_form(form) = %x{
    parts := make([]string, 0, int(form.live))
    for k, hv := range rbHashPairs(form) {
      parts = append(parts, rbFormEscape(string(k))+"="+rbFormEscape(string(hv)))
    }
    return String(strings.Join(parts, "&"))
  }

  #: (String) -> String
  def self.encode_www_form_component(s) = %x{ String(rbFormEscape(string(s))) }

  # Strings are UTF-8 (decision 136), so the encoding rack passes is the only one there is.
  #: (String, ?Encoding) -> String
  def self.decode_www_form_component(s, _enc = Encoding::UTF_8) = %x{
    out, err := url.QueryUnescape(string(s))
    if err != nil {
      panic(NewArgumentError(Ref(String("invalid %-encoding (" + string(s) + ")"))))
    }
    return String(out)
  }

  #: (String) -> Array[[String, String]]
  def self.decode_www_form(s) = %x{
    out := NewArray[Tuple2[String, String]]()
    if len(s) == 0 {
      return out
    }
    for _, part := range strings.Split(string(s), "&") {
      k, v, _ := strings.Cut(part, "=")
      dk, err := url.QueryUnescape(k)
      if err != nil {
        panic(NewArgumentError(Ref(String("invalid %-encoding (" + string(s) + ")"))))
      }
      dv, err := url.QueryUnescape(v)
      if err != nil {
        panic(NewArgumentError(Ref(String("invalid %-encoding (" + string(s) + ")"))))
      }
      Array_Push(out, Tuple2[String, String]{String(dk), String(dv)})
    }
    return out
  }
end

#: (String) -> URI::Generic
def URI(str) = URI.parse(str)
