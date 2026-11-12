package compiler

import "context"

func (c *Compiler) link(ctx context.Context) {
	for _, cls := range c.classList {
		if sup := c.superclassOf(cls); sup != nil {
			cls.Super = sup
			sup.Subclasses = append(sup.Subclasses, cls)
		}
		for i := range cls.Includes {
			c.linkInclude(cls, &cls.Includes[i])
		}
	}
	c.checkCycles()
	c.checkInstanceBounds()
	c.checkNumeric()
	c.buildMetas()
	c.declareLatentIvars()
	c.applyAliasNews(ctx)
	c.checkSpecUses() // before signatures: a bad let name would otherwise surface as an override mismatch
	for _, cls := range c.classList {
		if cls.GoType != "" && cls.Super != nil && cls.Super.isStruct() && !cls.Super.universal {
			c.errorAt(cls.srcPos, "@go_type class %s cannot inherit from struct class %s", cls.Name, cls.Super.Name)
		}
		if cls.isStruct() && !cls.universal && len(cls.TypeParams) > 0 && !cls.isTemplate() && !cls.hashBase {
			c.errorAt(cls.srcPos, "generic struct classes are not supported")
		}
	}
	c.addSigIvars()
	c.inheritRedeclaredReaders()
	c.declareIvarAnnotations()
	c.checkIvarModules()
	c.expandDelegations(ctx)
	c.applyPrepends()
	c.copyIncluderTemplates()
	c.linkAliases(ctx)
	c.separateFreeFuncs()
	c.methodClassVars()
	c.userGlobals()
	c.includeFromEach() // before any override inherits from the module, which needs its type args
	// method signatures
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			c.resolveMethod(m)
			nameCmpNil(m)
		}
	}
	for _, m := range c.topDefList {
		c.resolveMethod(m)
	}
	c.markOutParams()
	c.markCalleeDefaults()
	c.markSuperBridges()
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			switch {
			case c.overridesIterator(m) || c.blockEachOfEnumerable(cls, m):
				m.seqAdapter = true
				m.GoName += "_blk"
			case c.restEachOfEnumerable(cls, m):
				m.seqRest = true
				m.GoName += "_rest"
			case protoOptShape(m):
				m.protoOpt = true
				m.GoName += "_opt"
			}
		}
	}
	c.publicGemOverrides()
	for _, cls := range c.classList {
		c.linkOverrides(cls)
	}
	for _, cls := range c.classList {
		ms := make([]*Method, 0, len(cls.MethodList))
		for _, e := range cls.methodSet() {
			ms = append(ms, e.M)
		}
		c.checkGoNames(cls.RubyName, ms)
	}
	c.checkGoNames("the top level", c.topDefList)
}
