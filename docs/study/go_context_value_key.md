# Go `context.Context` Value Key 패턴

## 질문

```go
type contextAttrsKey struct{}
```

변수도 없는데 이 타입이 어떻게 `context.Context`의 key로 동작하는가?

## 핵심

`type` 자체가 key인 것은 아니다.
실제 key는 `contextAttrsKey{}` 표현식으로 즉석에서 생성한 **값**이다.

```go
type contextAttrsKey struct{}

ctx = context.WithValue(ctx, contextAttrsKey{}, merged)
attrs := ctx.Value(contextAttrsKey{})
```

위 코드를 변수로 풀어 쓰면 다음과 같다.

```go
saveKey := contextAttrsKey{}
ctx = context.WithValue(ctx, saveKey, merged)

lookupKey := contextAttrsKey{}
attrs := ctx.Value(lookupKey)
```

## 조회가 가능한 이유

`Context`는 저장했던 key의 주소를 찾지 않는다.
조회 시 전달된 key와 저장된 key를 `==`로 비교한다.

개념적으로는 다음과 비슷하다.

```go
type valueCtx struct {
	parent context.Context
	key    any
	val    any
}

func (c *valueCtx) Value(key any) any {
	if c.key == key {
		return c.val
	}
	return c.parent.Value(key)
}
```

빈 struct는 비교 가능한 값이며 내부 필드가 없다.
따라서 같은 타입의 빈 struct 값은 항상 같다.

```go
a := contextAttrsKey{}
b := contextAttrsKey{}

fmt.Println(a == b) // true
```

## 전용 타입을 사용하는 이유

문자열 key는 다른 패키지와 충돌할 수 있다.

```go
context.WithValue(ctx, "attrs", merged)
```

반면 unexported 전용 타입은 다른 패키지가 동일한 이름과 모양의 타입을 선언해도 별개의 타입이다.

```go
// package observability
type contextAttrsKey struct{}

// package other
type contextAttrsKey struct{} // 이름과 모양이 같아도 다른 타입
```

## 현재 프로젝트 적용 위치

`internal/observability/context.go`는 다음 용도로 이 패턴을 사용한다.

- `WithAttrs`: Context에 `[]slog.Attr` 저장
- `attrsFromContext`: 동일 key 값으로 attributes 조회
- `InteractionID`: 조회한 attributes에서 `interaction_id` 추출

## 정리

- 타입 정의: `type contextAttrsKey struct{}`
- 실제 key 값 생성: `contextAttrsKey{}`
- 조회 방식: 주소가 아니라 key 값의 `==` 비교
- 빈 struct 사용 이유: 값이 가볍고 같은 타입의 값끼리 항상 동일
- 전용 타입 사용 이유: 다른 패키지의 Context key와 충돌 방지
