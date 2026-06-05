print("case:oop_class_methods_more")

function Class(parent)
    local cls = {}
    cls.__index = cls
    if parent ~= nil then
        setmetatable(cls, {__index = parent})
    end
    cls.new = function(...)
        local instance = {}
        setmetatable(instance, cls)
        if cls.init ~= nil then
            cls.init(instance, ...)
        end
        return instance
    end
    return cls
end

Animal = Class(nil)
Animal.init = function(self, name)
    self.name = name
end
Animal.speak = function(self)
    return self.name .. " makes a sound"
end

Dog = Class(Animal)
Dog.init = function(self, name)
    Animal.init(self, name)
    self.tricks = {}
end
Dog.speak = function(self)
    return self.name .. " says woof"
end
Dog.learn = function(self, trick)
    table.insert(self.tricks, trick)
end
Dog.trickCount = function(self)
    return #self.tricks
end

function counter(start)
    local value = start
    return {
        add = function(self, n)
            value = value + n
            return value
        end,
        get = function(self)
            return value
        end,
    }
end

Named = {
    label = function(self)
        return "named:" .. self.name
    end,
}

for key, fn in pairs(Named) do
    Dog[key] = fn
end

rex = Dog.new("Rex")
print(rex:speak())
rex:learn("sit")
rex:learn("stay")
assert(rex:trickCount() == 2)
print(rex:label())

c = counter(10)
assert(c:add(5) == 15)
assert(c:get() == 15)
print("counter:" .. c:get())

print("ok")
