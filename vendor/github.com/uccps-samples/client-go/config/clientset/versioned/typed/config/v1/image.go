// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	v1 "github.com/uccps-samples/api/config/v1"
	scheme "github.com/uccps-samples/client-go/config/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ImagesGetter has a method to return a ImageInterface.
// A group's client should implement this interface.
type ImagesGetter interface {
	Images() ImageInterface
}

// ImageInterface has methods to work with Image resources.
type ImageInterface interface {
	Create(ctx context.Context, image *v1.Image, opts metav1.CreateOptions) (*v1.Image, error)
	Update(ctx context.Context, image *v1.Image, opts metav1.UpdateOptions) (*v1.Image, error)
	UpdateStatus(ctx context.Context, image *v1.Image, opts metav1.UpdateOptions) (*v1.Image, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Image, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.ImageList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.Image, err error)
	ImageExpansion
}

// images implements ImageInterface
type images struct {
	client rest.Interface
}

// newImages returns a Images
func newImages(c *ConfigV1Client) *images {
	return &images{
		client: c.RESTClient(),
	}
}

// Get takes name of the image, and returns the corresponding image object, and an error if there is any.
func (c *images) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Get().
		Resource("images").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Images that match those selectors.
func (c *images) List(ctx context.Context, opts metav1.ListOptions) (result *v1.ImageList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.ImageList{}
	err = c.client.Get().
		Resource("images").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested images.
func (c *images) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("images").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a image and creates it.  Returns the server's representation of the image, and an error, if there is any.
func (c *images) Create(ctx context.Context, image *v1.Image, opts metav1.CreateOptions) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Post().
		Resource("images").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(image).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a image and updates it. Returns the server's representation of the image, and an error, if there is any.
func (c *images) Update(ctx context.Context, image *v1.Image, opts metav1.UpdateOptions) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Put().
		Resource("images").
		Name(image.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(image).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *images) UpdateStatus(ctx context.Context, image *v1.Image, opts metav1.UpdateOptions) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Put().
		Resource("images").
		Name(image.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(image).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the image and deletes it. Returns an error if one occurs.
func (c *images) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Resource("images").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *images) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("images").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched image.
func (c *images) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Patch(pt).
		Resource("images").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
