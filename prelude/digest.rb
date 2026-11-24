# prelude/digest.rb
# rbs_inline: enabled

#
# Digest::MD5/SHA1/SHA256/SHA384/SHA512 on crypto/*. Loads only on a literal `require "digest"` (or digest/md5, sha1, sha2; decision 155).
# `new` returns a Digest::Base holding the bytes fed to it; the sum is
# taken when asked for.

module Digest
  # @go_type struct { algo string; buf []byte }
  class Base < Object
    #: (String) -> Base
    def self.__for(algo) = %x{ return &Digest_Base{algo: string(algo)} }

    #: (String) -> Base
    def update(s) = %x{
      self.buf = append(self.buf, s...)
      return self
    }

    #: (String) -> Base
    def <<(s) = update(s)

    # MRI's Digest::Instance#file: the file's bytes fed in, self back (rubocop's ResultCache#file_checksum).
    #: (String) -> Base
    def file(path) = update(File.binread(path))

    #: () -> Base
    def reset = %x{
      self.buf = self.buf[:0]
      return self
    }

    # @binary
    #: () -> String
    def digest = %x{ String(rbDigestSum(self.algo, self.buf)) }

    #: () -> String
    def hexdigest = %x{ String(hex.EncodeToString(rbDigestSum(self.algo, self.buf))) }

    #: () -> String
    def base64digest = %x{ String(base64.StdEncoding.EncodeToString(rbDigestSum(self.algo, self.buf))) }

    #: () -> String
    def to_s = hexdigest

    #: () -> String
    def name = %x{ String(self.algo) }

    #: () -> String
    def inspect = "#<Digest::#{name}: #{hexdigest}>"

    #: (untyped) -> bool
    def ==(other) = %x{
      o, ok := other.(*Digest_Base)
      return Boolean(ok && bytes.Equal(rbDigestSum(self.algo, self.buf), rbDigestSum(o.algo, o.buf)))
    }
  end

  #: (String) -> String
  def self.hexencode(s) = %x{ String(hex.EncodeToString([]byte(s))) }

  class MD5 < Object
    #: () -> Base
    def self.new = Base.__for("MD5")
    # @binary
    #: (String) -> String
    def self.digest(s) = Base.__for("MD5").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("MD5").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("MD5").update(s).base64digest
    #: (String) -> Base
    def self.file(path) = Base.__for("MD5").update(File.read(path))
  end

  class SHA1 < Object
    #: () -> Base
    def self.new = Base.__for("SHA1")
    # @binary
    #: (String) -> String
    def self.digest(s) = Base.__for("SHA1").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("SHA1").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("SHA1").update(s).base64digest
    #: (String) -> Base
    def self.file(path) = Base.__for("SHA1").update(File.read(path))
  end

  class SHA256 < Object
    #: () -> Base
    def self.new = Base.__for("SHA256")
    # @binary
    #: (String) -> String
    def self.digest(s) = Base.__for("SHA256").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("SHA256").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("SHA256").update(s).base64digest
    #: (String) -> Base
    def self.file(path) = Base.__for("SHA256").update(File.read(path))
  end

  class SHA384 < Object
    #: () -> Base
    def self.new = Base.__for("SHA384")
    # @binary
    #: (String) -> String
    def self.digest(s) = Base.__for("SHA384").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("SHA384").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("SHA384").update(s).base64digest
    #: (String) -> Base
    def self.file(path) = Base.__for("SHA384").update(File.read(path))
  end

  class SHA512 < Object
    #: () -> Base
    def self.new = Base.__for("SHA512")
    # @binary
    #: (String) -> String
    def self.digest(s) = Base.__for("SHA512").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("SHA512").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("SHA512").update(s).base64digest
    #: (String) -> Base
    def self.file(path) = Base.__for("SHA512").update(File.read(path))
  end

  # SHA2.new(bitlen): 256/384/512, defaulting to 256, as MRI's.
  class SHA2 < Object
    #: (?Integer) -> Base
    def self.new(bitlen = 256) = %x{
      switch bitlen {
      case 256, 384, 512:
        return &Digest_Base{algo: fmt.Sprintf("SHA%d", bitlen)}
      }
      panic(NewArgumentError(Ref(String(fmt.Sprintf("unsupported bit length: %d", bitlen)))))
    }

    # @binary
    #: (String) -> String
    def self.digest(s) = Base.__for("SHA256").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("SHA256").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("SHA256").update(s).base64digest
    #: (String) -> Base
    def self.file(path) = Base.__for("SHA256").update(File.read(path))
  end
end
